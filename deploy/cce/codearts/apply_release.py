#!/usr/bin/env python3
"""Apply a rendered release ConfigMap and Job with a least-privilege token."""

from __future__ import annotations

import argparse
import base64
import json
import os
from pathlib import Path
import re
import ssl
import urllib.error
import urllib.request


NAME_RE = re.compile(r"^sub2api-release-[a-z0-9-]+$")
IMAGE_RE = re.compile(
    r"^registry\.cn-shanghai\.aliyuncs\.com/rayless/sub2api@sha256:[0-9a-f]{64}$"
)


def validate_workload(value: dict) -> tuple[dict, dict]:
    if value.get("apiVersion") != "v1" or value.get("kind") != "List":
        raise ValueError("release workload must be a Kubernetes v1 List")
    items = value.get("items")
    if not isinstance(items, list) or len(items) != 2:
        raise ValueError("release workload must contain one ConfigMap and one Job")
    by_kind = {item.get("kind"): item for item in items if isinstance(item, dict)}
    if set(by_kind) != {"ConfigMap", "Job"}:
        raise ValueError("release workload kinds must be ConfigMap and Job")
    configmap = by_kind["ConfigMap"]
    job = by_kind["Job"]
    for item in (configmap, job):
        metadata = item.get("metadata", {})
        if metadata.get("namespace") != "sub2api-prod":
            raise ValueError("release resources must use sub2api-prod")
        if metadata.get("labels", {}).get("app.kubernetes.io/name") != "sub2api-release":
            raise ValueError("release resources must carry the release application label")
        if not NAME_RE.fullmatch(str(metadata.get("name", ""))):
            raise ValueError("release resource name is unsafe")
    pod_spec = job["spec"]["template"]["spec"]
    if pod_spec.get("serviceAccountName") != "sub2api-release":
        raise ValueError("release Job must use the release ServiceAccount")
    containers = pod_spec.get("containers", [])
    if len(containers) != 1 or not IMAGE_RE.fullmatch(str(containers[0].get("image", ""))):
        raise ValueError("release Job must use one immutable production runner image")
    return configmap, job


class KubernetesClient:
    def __init__(self, server: str, token: str, ca_data: str):
        if not server.startswith("https://"):
            raise ValueError("Kubernetes server must use HTTPS")
        self.server = server.rstrip("/")
        self.token = token
        ca_pem = base64.b64decode(ca_data, validate=True).decode("ascii")
        self.context = ssl.create_default_context(cadata=ca_pem)

    def request(self, method: str, path: str, body: dict | None = None) -> tuple[int, dict]:
        data = None if body is None else json.dumps(body).encode("utf-8")
        headers = {"Authorization": f"Bearer {self.token}", "Accept": "application/json"}
        if data is not None:
            headers["Content-Type"] = (
                "application/merge-patch+json" if method == "PATCH" else "application/json"
            )
        request = urllib.request.Request(
            self.server + path, data=data, headers=headers, method=method
        )
        try:
            with urllib.request.urlopen(request, timeout=30, context=self.context) as response:
                return response.status, json.load(response)
        except urllib.error.HTTPError as error:
            payload = json.loads(error.read().decode("utf-8") or "{}")
            return error.code, payload

    def apply(self, kind: str, value: dict) -> str:
        name = value["metadata"]["name"]
        namespace = value["metadata"]["namespace"]
        if kind == "ConfigMap":
            collection = f"/api/v1/namespaces/{namespace}/configmaps"
        else:
            collection = f"/apis/batch/v1/namespaces/{namespace}/jobs"
        status, current = self.request("GET", f"{collection}/{name}")
        if status == 404:
            status, result = self.request("POST", collection, value)
            if status not in (200, 201):
                raise RuntimeError(f"create {kind}/{name} failed with HTTP {status}: {result}")
            return "created"
        if status != 200:
            raise RuntimeError(f"get {kind}/{name} failed with HTTP {status}: {current}")
        if kind == "Job":
            raise RuntimeError(f"Job/{name} already exists; refusing an ambiguous replay")
        patch = {"metadata": {"labels": value["metadata"].get("labels", {})}, "data": value["data"]}
        status, result = self.request("PATCH", f"{collection}/{name}", patch)
        if status != 200:
            raise RuntimeError(f"patch {kind}/{name} failed with HTTP {status}: {result}")
        return "patched"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--workload", type=Path, required=True)
    args = parser.parse_args()
    workload = json.loads(args.workload.read_text(encoding="utf-8"))
    configmap, job = validate_workload(workload)
    client = KubernetesClient(
        os.environ["K8S_API_SERVER"], os.environ["K8S_BEARER_TOKEN"], os.environ["K8S_CA_DATA"]
    )
    for kind, item in (("ConfigMap", configmap), ("Job", job)):
        action = client.apply(kind, item)
        print(f"{action} {kind}/{item['metadata']['name']}", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
