#!/usr/bin/env python3
"""Deploy one immutable Sub2API CCE release from an ACR manifest.

This tool intentionally runs on the operations host. GitHub builds images and
produces the manifest, but never receives cluster or SSH credentials.
"""

from __future__ import annotations

import argparse
import copy
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid

try:
    import fcntl
except ImportError:  # pragma: no cover - deployment runs on Linux; enables Windows unit tests
    fcntl = None


IMAGE_RE = re.compile(
    r"^registry\.cn-shanghai\.aliyuncs\.com/rayless/sub2api@sha256:[0-9a-f]{64}$"
)
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
SAFE_VALUE_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$")
ANNOTATION_PREFIX = "sub2api.2ray.wang/"
PRODUCTION_INSIGHTS_STATISTICS_START_DATE = "2026-06-01"


class ReleaseError(RuntimeError):
    pass


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def run(command: list[str], *, timeout: int = 180, check: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(command, text=True, capture_output=True, timeout=timeout, check=False)
    if check and result.returncode:
        stderr = result.stderr.strip()
        raise ReleaseError(f"command failed ({result.returncode}): {' '.join(command)}: {stderr}")
    return result


def save_json(path: Path, value: object) -> None:
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    path.chmod(0o600)


def load_manifest(path: Path) -> dict:
    data = json.loads(path.read_text(encoding="utf-8"))
    if data.get("schema_version") != "2":
        raise ReleaseError("release manifest schema_version must be 2")
    source_sha = str(data.get("source_sha", ""))
    source_ref = str(data.get("source_ref", ""))
    version = str(data.get("version", ""))
    if not SHA_RE.fullmatch(source_sha):
        raise ReleaseError("manifest source_sha must be a full lowercase commit SHA")
    if not SAFE_VALUE_RE.fullmatch(source_ref) or ".." in source_ref or "//" in source_ref:
        raise ReleaseError("manifest source_ref is unsafe")
    if not SAFE_VALUE_RE.fullmatch(version):
        raise ReleaseError("manifest version is unsafe")
    images = data.get("images")
    if not isinstance(images, dict) or set(images) != {"backend", "insights"}:
        raise ReleaseError("manifest must contain backend and insights images")
    for component, image in images.items():
        if not isinstance(image, dict) or not IMAGE_RE.fullmatch(str(image.get("immutable_image", ""))):
            raise ReleaseError(f"manifest {component} image is not an immutable production ACR digest")
        if image.get("source_sha") != source_sha:
            raise ReleaseError(f"manifest {component} source SHA does not match release source")
    return data


def dedupe_pull_secrets(pod_spec: dict, name: str = "acr-pull") -> None:
    names: list[str] = []
    for item in [{"name": name}, *pod_spec.get("imagePullSecrets", [])]:
        value = item.get("name")
        if value and value not in names:
            names.append(value)
    pod_spec["imagePullSecrets"] = [{"name": value} for value in names]


def named_container(pod_spec: dict, name: str) -> dict:
    for container in pod_spec.get("containers", []):
        if container.get("name") == name:
            return container
    raise ReleaseError(f"container {name!r} is missing from deployment")


def upsert_env(container: dict, name: str, value: str) -> None:
    env = container.setdefault("env", [])
    for item in env:
        if item.get("name") == name:
            item.clear()
            item.update({"name": name, "value": value})
            return
    env.append({"name": name, "value": value})


def remove_env(container: dict, name: str) -> None:
    container["env"] = [item for item in container.get("env", []) if item.get("name") != name]


def release_annotations(manifest: dict, token: str) -> dict[str, str]:
    return {
        f"{ANNOTATION_PREFIX}gha-run": str(manifest.get("build_run_id", "")),
        f"{ANNOTATION_PREFIX}release-token": token,
        f"{ANNOTATION_PREFIX}source-ref": manifest["source_ref"],
        f"{ANNOTATION_PREFIX}source-revision": manifest["source_sha"],
        f"{ANNOTATION_PREFIX}version": manifest["version"],
    }


def build_backend_patch(deployment: dict, image: str, annotations: dict[str, str]) -> dict:
    pod_spec = copy.deepcopy(deployment["spec"]["template"]["spec"])
    container = named_container(pod_spec, "sub2api")
    container["image"] = image
    container["imagePullPolicy"] = "IfNotPresent"
    container["lifecycle"] = {"preStop": {"exec": {"command": ["/bin/sh", "-c", "sleep 5"]}}}
    upsert_env(container, "SERVER_SHUTDOWN_TIMEOUT_SECONDS", "60")
    upsert_env(container, "INSIGHTS_TRUSTED_COLLECTION", "true")
    remove_env(container, "INSIGHTS_TRUSTED_USAGE_HISTORY_FROM")
    upsert_env(
        container,
        "INSIGHTS_STATISTICS_START_DATE",
        PRODUCTION_INSIGHTS_STATISTICS_START_DATE,
    )
    pod_spec["terminationGracePeriodSeconds"] = max(75, int(pod_spec.get("terminationGracePeriodSeconds", 0)))
    dedupe_pull_secrets(pod_spec)
    return {
        "metadata": {"annotations": annotations},
        "spec": {
            "minReadySeconds": 5,
            "progressDeadlineSeconds": 240,
            "strategy": {"type": "RollingUpdate", "rollingUpdate": {"maxSurge": 1, "maxUnavailable": 0}},
            "template": {"metadata": {"annotations": annotations}, "spec": pod_spec},
        },
    }


def build_insights_patch(
    deployment: dict, image: str, annotations: dict[str, str], configmap: str
) -> dict:
    pod_spec = copy.deepcopy(deployment["spec"]["template"]["spec"])
    container = named_container(pod_spec, "insights")
    container["image"] = image
    container["imagePullPolicy"] = "IfNotPresent"
    container["volumeMounts"] = [
        mount for mount in container.get("volumeMounts", []) if mount.get("name") != "previous-assets"
    ]
    pod_spec["volumes"] = [
        volume for volume in pod_spec.get("volumes", []) if volume.get("name") != "previous-assets"
    ]
    config_volume = next((volume for volume in pod_spec["volumes"] if volume.get("name") == "config"), None)
    if config_volume is None:
        raise ReleaseError("Insights config volume is missing")
    config_volume["configMap"] = {"name": configmap}
    pod_spec["initContainers"] = None
    dedupe_pull_secrets(pod_spec)
    direct_annotations = dict(annotations)
    direct_annotations[f"{ANNOTATION_PREFIX}release-stage"] = "direct"
    return {
        "metadata": {"annotations": annotations},
        "spec": {
            "minReadySeconds": 3,
            "progressDeadlineSeconds": 180,
            "strategy": {"type": "RollingUpdate", "rollingUpdate": {"maxSurge": 1, "maxUnavailable": 0}},
            "template": {"metadata": {"annotations": direct_annotations}, "spec": pod_spec},
        },
    }


class Controller:
    def __init__(self, context: str, namespace: str, audit_dir: Path):
        self.context = context
        self.namespace = namespace
        self.audit_dir = audit_dir
        self.kubectl = ["kubectl", "--context", context, "-n", namespace]

    def get(self, kind: str, name: str) -> dict:
        result = run([*self.kubectl, "get", kind, name, "-o", "json"])
        return json.loads(result.stdout)

    def patch_deployment(self, name: str, patch: dict) -> None:
        run([*self.kubectl, "patch", "deployment", name, "--type", "merge", "-p", json.dumps(patch)])

    def rollout_status(self, name: str, timeout: int) -> None:
        run([*self.kubectl, "rollout", "status", f"deployment/{name}", f"--timeout={timeout}s"], timeout=timeout + 15)

    def rollback(self, name: str, revision: str) -> None:
        run([*self.kubectl, "rollout", "undo", f"deployment/{name}", f"--to-revision={revision}"])
        self.rollout_status(name, 300)

    def pods_for(self, deployment: dict) -> list[dict]:
        labels = deployment["spec"]["selector"]["matchLabels"]
        selector = ",".join(f"{key}={value}" for key, value in sorted(labels.items()))
        result = run([*self.kubectl, "get", "pods", "-l", selector, "-o", "json"])
        return json.loads(result.stdout)["items"]

    def assert_service_selector(self, name: str) -> None:
        selector = self.get("service", name)["spec"].get("selector", {})
        if "pod-template-hash" in selector:
            raise ReleaseError(f"service {name} is pinned to a rollout hash")

    def validate_deployment(self, name: str, container_name: str, image: str, direct_insights: bool = False) -> dict:
        deployment = self.get("deployment", name)
        desired = int(deployment["spec"].get("replicas", 1))
        status = deployment.get("status", {})
        generation = int(deployment["metadata"]["generation"])
        if int(status.get("observedGeneration", 0)) < generation:
            raise ReleaseError(f"deployment {name} has not observed generation {generation}")
        for field in ("replicas", "updatedReplicas", "readyReplicas", "availableReplicas"):
            if int(status.get(field, 0)) != desired:
                raise ReleaseError(f"deployment {name} {field} is not {desired}")
        pod_spec = deployment["spec"]["template"]["spec"]
        if named_container(pod_spec, container_name).get("image") != image:
            raise ReleaseError(f"deployment {name} does not use expected immutable image")
        if direct_insights:
            if pod_spec.get("initContainers"):
                raise ReleaseError("Insights deployment still has compatibility init containers")
            if any(item.get("name") == "previous-assets" for item in pod_spec.get("volumes", [])):
                raise ReleaseError("Insights deployment still has a previous-assets volume")
            mounts = named_container(pod_spec, container_name).get("volumeMounts", [])
            if any(item.get("name") == "previous-assets" for item in mounts):
                raise ReleaseError("Insights deployment still mounts previous assets")
        pods = [pod for pod in self.pods_for(deployment) if not pod["metadata"].get("deletionTimestamp")]
        if len(pods) != desired:
            raise ReleaseError(f"deployment {name} has {len(pods)} pods, expected {desired}")
        pod_summary = []
        for pod in pods:
            statuses = pod.get("status", {}).get("containerStatuses", [])
            if not statuses or any(not item.get("ready") or item.get("restartCount") != 0 for item in statuses):
                raise ReleaseError(f"pod {pod['metadata']['name']} is not cleanly ready")
            pod_summary.append({"name": pod["metadata"]["name"], "restarts": 0})
        return {"deployment": name, "replicas": desired, "pods": pod_summary}


def probe(url: str, expected: int) -> bytes:
    request = urllib.request.Request(url, headers={"User-Agent": "sub2api-cce-release/1"})
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            status = response.status
            body = response.read()
    except urllib.error.HTTPError as error:
        status = error.code
        body = error.read()
    if status != expected:
        raise ReleaseError(f"public probe {url} returned {status}, expected {expected}")
    return body


def public_checks() -> dict:
    checks = {
        "https://ai.2ray.wang/": 200,
        "https://ai.2ray.wang/login": 200,
        "https://ai.2ray.wang/insights/": 200,
        "https://ai.2ray.wang/insights/models": 200,
        "https://sub2api.2ray.wang/health": 200,
        "https://sub2api.2ray.wang/api/v1/settings/public": 200,
        "https://sub2api.2ray.wang/api/v1/insights/models?window=24h": 401,
    }
    bodies = {url: probe(url, expected) for url, expected in checks.items()}
    index = bodies["https://ai.2ray.wang/insights/"]
    match = re.search(rb'/insights/assets/index-[A-Za-z0-9._-]+\.js', index)
    if not match:
        raise ReleaseError("Insights index does not reference a versioned application asset")
    asset = match.group(0).decode("ascii")
    probe("https://ai.2ray.wang" + asset, 200)
    return {
        "checks": len(checks) + 1,
        "index_sha256": hashlib.sha256(index).hexdigest(),
        "asset": asset,
    }


def revision(deployment: dict) -> str:
    value = deployment["metadata"].get("annotations", {}).get("deployment.kubernetes.io/revision")
    if not value or not value.isdigit():
        raise ReleaseError(f"deployment {deployment['metadata']['name']} has no rollout revision")
    return value


def release(args: argparse.Namespace) -> dict:
    manifest = load_manifest(args.manifest.resolve())
    token = uuid.uuid4().hex[:24]
    release_name = f"{dt.datetime.now(dt.timezone.utc):%Y%m%dT%H%M%SZ}-{manifest['source_sha'][:12]}"
    audit_dir = args.audit_root.expanduser().resolve() / release_name
    audit_dir.mkdir(parents=True, mode=0o700)
    controller = Controller(args.context, args.namespace, audit_dir)
    annotations = release_annotations(manifest, token)

    backend_before = controller.get("deployment", args.backend_deployment)
    insights_before = controller.get("deployment", args.insights_deployment)
    for deployment in (backend_before, insights_before):
        current_sha = deployment["metadata"].get("annotations", {}).get(f"{ANNOTATION_PREFIX}source-revision")
        if args.expected_current_sha and current_sha != args.expected_current_sha:
            raise ReleaseError(
                f"{deployment['metadata']['name']} source changed: {current_sha!r} != {args.expected_current_sha!r}"
            )
    controller.assert_service_selector(args.backend_service)
    controller.assert_service_selector(args.insights_service)
    save_json(audit_dir / "manifest.json", manifest)
    save_json(audit_dir / "backend-before.json", backend_before)
    save_json(audit_dir / "insights-before.json", insights_before)

    backend_image = manifest["images"]["backend"]["immutable_image"]
    insights_image = manifest["images"]["insights"]["immutable_image"]
    backend_patch = build_backend_patch(backend_before, backend_image, annotations)
    insights_patch = build_insights_patch(
        insights_before, insights_image, annotations, args.insights_configmap
    )
    save_json(audit_dir / "backend-patch.json", backend_patch)
    save_json(audit_dir / "insights-patch.json", insights_patch)
    if args.dry_run:
        return {"phase": "dry_run_complete", "audit_dir": str(audit_dir), "release_token": token}

    backend_started = False
    insights_started = False
    try:
        controller.patch_deployment(args.backend_deployment, backend_patch)
        backend_started = True
        controller.rollout_status(args.backend_deployment, 300)
        backend_state = controller.validate_deployment(
            args.backend_deployment, "sub2api", backend_image
        )
        probe("https://sub2api.2ray.wang/health", 200)
        probe("https://sub2api.2ray.wang/api/v1/settings/public", 200)

        controller.patch_deployment(args.insights_deployment, insights_patch)
        insights_started = True
        controller.rollout_status(args.insights_deployment, 240)
        insights_state = controller.validate_deployment(
            args.insights_deployment, "insights", insights_image, direct_insights=True
        )

        samples = []
        deadline = time.monotonic() + args.observation_seconds
        while True:
            backend_state = controller.validate_deployment(
                args.backend_deployment, "sub2api", backend_image
            )
            insights_state = controller.validate_deployment(
                args.insights_deployment, "insights", insights_image, direct_insights=True
            )
            sample = {"at": utc_now(), **public_checks()}
            samples.append(sample)
            if time.monotonic() >= deadline:
                break
            time.sleep(min(3, max(0, deadline - time.monotonic())))

        result = {
            "phase": "success",
            "at": utc_now(),
            "audit_dir": str(audit_dir),
            "source_sha": manifest["source_sha"],
            "version": manifest["version"],
            "backend": backend_state,
            "insights": insights_state,
            "observation_samples": len(samples),
            "public": samples[-1],
        }
        save_json(audit_dir / "success.json", result)
        save_json(audit_dir / "probes.json", samples)
        return result
    except Exception as error:
        rollback_errors = []
        for started, name, before in (
            (insights_started, args.insights_deployment, insights_before),
            (backend_started, args.backend_deployment, backend_before),
        ):
            if not started:
                continue
            try:
                controller.rollback(name, revision(before))
            except Exception as rollback_error:  # noqa: BLE001 - preserve both failures
                rollback_errors.append(f"{name}: {rollback_error}")
        failure = {
            "phase": "rolled_back" if not rollback_errors else "rollback_incomplete",
            "at": utc_now(),
            "error": str(error),
            "rollback_errors": rollback_errors,
        }
        save_json(audit_dir / "failure.json", failure)
        raise ReleaseError(json.dumps(failure, sort_keys=True)) from error


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    result.add_argument("--manifest", type=Path, required=True)
    result.add_argument("--context", default="hispark-cce")
    result.add_argument("--namespace", default="sub2api-prod")
    result.add_argument("--backend-deployment", default="sub2api")
    result.add_argument("--backend-service", default="sub2api")
    result.add_argument("--insights-deployment", default="sub2api-insights")
    result.add_argument("--insights-service", default="sub2api-insights")
    result.add_argument("--insights-configmap", default="sub2api-insights-nginx")
    result.add_argument("--expected-current-sha", default="")
    result.add_argument("--observation-seconds", type=int, default=30)
    result.add_argument("--audit-root", type=Path, default=Path.home() / "sub2api-releases")
    result.add_argument("--dry-run", action="store_true")
    return result


def main() -> int:
    args = parser().parse_args()
    if args.observation_seconds < 0 or args.observation_seconds > 600:
        raise ReleaseError("observation-seconds must be between 0 and 600")
    if fcntl is None:
        raise ReleaseError("the CCE release command must run on Linux")
    lock_path = Path(os.environ.get("SUB2API_CCE_RELEASE_LOCK", "/tmp/sub2api-cce-release.lock"))
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    with lock_path.open("w", encoding="utf-8") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise ReleaseError("another CCE release is already running") from error
        print(json.dumps(release(args), sort_keys=True), flush=True)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (ReleaseError, json.JSONDecodeError, OSError, subprocess.TimeoutExpired) as error:
        print(f"release failed: {error}", file=sys.stderr)
        raise SystemExit(1)
