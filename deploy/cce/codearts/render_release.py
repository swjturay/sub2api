#!/usr/bin/env python3
"""Render the immutable release manifest and the CodeArts CCE Job workload."""

from __future__ import annotations

import argparse
import datetime as dt
import json
from pathlib import Path
import re


IMAGE_RE = re.compile(
    r"^registry\.cn-shanghai\.aliyuncs\.com/rayless/sub2api@sha256:[0-9a-f]{64}$"
)
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
VERSION_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$")
RUN_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,39}$")


def safe_name(value: str) -> str:
    value = re.sub(r"[^a-z0-9-]+", "-", value.lower()).strip("-")
    return value[:40] or "manual"


def render(args: argparse.Namespace) -> tuple[dict, dict]:
    if not SHA_RE.fullmatch(args.source_sha):
        raise ValueError("source SHA must be a full lowercase commit SHA")
    if not VERSION_RE.fullmatch(args.version):
        raise ValueError("version is not a safe image tag component")
    if not RUN_RE.fullmatch(args.pipeline_run):
        raise ValueError("pipeline run identifier is unsafe")
    for name, image in (
        ("backend", args.backend_image),
        ("insights", args.insights_image),
        ("runner", args.runner_image),
    ):
        if not IMAGE_RE.fullmatch(image):
            raise ValueError(f"{name} image is not an immutable production ACR digest")

    created_at = dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat()
    manifest = {
        "schema_version": "2",
        "created_at": created_at,
        "source_repository": "https://github.com/swjturay/sub2api",
        "source_ref": "cce-deploy",
        "source_sha": args.source_sha,
        "version": args.version,
        "build_run_id": args.pipeline_run,
        "run_url": "CodeArts/HiSpark/sub2api-production",
        "images": {
            "backend": {
                "component": "backend",
                "immutable_image": args.backend_image,
                "digest": args.backend_image.rsplit("@", 1)[1],
                "platform": "linux/amd64",
                "source_sha": args.source_sha,
            },
            "insights": {
                "component": "insights",
                "immutable_image": args.insights_image,
                "digest": args.insights_image.rsplit("@", 1)[1],
                "platform": "linux/amd64",
                "source_sha": args.source_sha,
            },
        },
    }

    suffix = safe_name(f"{args.pipeline_run}-{args.source_sha[:10]}")
    job_name = f"sub2api-release-{suffix}"[:63].rstrip("-")
    config_name = f"{job_name}-manifest"[:63].rstrip("-")
    labels = {
        "app.kubernetes.io/name": "sub2api-release",
        "app.kubernetes.io/part-of": "sub2api",
        "sub2api.2ray.wang/source-revision": args.source_sha,
    }
    workload = {
        "apiVersion": "v1",
        "kind": "List",
        "items": [
            {
                "apiVersion": "v1",
                "kind": "ConfigMap",
                "metadata": {"name": config_name, "namespace": "sub2api-prod", "labels": labels},
                "data": {"cce-release-manifest.json": json.dumps(manifest, sort_keys=True)},
            },
            {
                "apiVersion": "batch/v1",
                "kind": "Job",
                "metadata": {"name": job_name, "namespace": "sub2api-prod", "labels": labels},
                "spec": {
                    "backoffLimit": 0,
                    "activeDeadlineSeconds": 1200,
                    "ttlSecondsAfterFinished": 60,
                    "template": {
                        "metadata": {"labels": labels},
                        "spec": {
                            "serviceAccountName": "sub2api-release",
                            "restartPolicy": "Never",
                            "imagePullSecrets": [{"name": "acr-pull"}],
                            "securityContext": {
                                "runAsNonRoot": True,
                                "runAsUser": 65532,
                                "runAsGroup": 65532,
                                "fsGroup": 65532,
                                "seccompProfile": {"type": "RuntimeDefault"},
                            },
                            "containers": [
                                {
                                    "name": "release",
                                    "image": args.runner_image,
                                    "imagePullPolicy": "IfNotPresent",
                                    "args": [
                                        "--manifest",
                                        "/release/cce-release-manifest.json",
                                        "--drain-seconds",
                                        "180",
                                    ],
                                    "securityContext": {
                                        "allowPrivilegeEscalation": False,
                                        "capabilities": {"drop": ["ALL"]},
                                        "readOnlyRootFilesystem": True,
                                    },
                                    "volumeMounts": [
                                        {"name": "manifest", "mountPath": "/release", "readOnly": True},
                                        {"name": "audit", "mountPath": "/audit"},
                                        {"name": "tmp", "mountPath": "/tmp"},
                                    ],
                                }
                            ],
                            "volumes": [
                                {"name": "manifest", "configMap": {"name": config_name}},
                                {"name": "audit", "emptyDir": {}},
                                {"name": "tmp", "emptyDir": {}},
                            ],
                        },
                    },
                },
            },
        ],
    }
    return manifest, workload


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    result.add_argument("--source-sha", required=True)
    result.add_argument("--version", required=True)
    result.add_argument("--pipeline-run", required=True)
    result.add_argument("--backend-image", required=True)
    result.add_argument("--insights-image", required=True)
    result.add_argument("--runner-image", required=True)
    result.add_argument("--output-dir", type=Path, required=True)
    return result


def main() -> int:
    args = parser().parse_args()
    manifest, workload = render(args)
    args.output_dir.mkdir(parents=True, exist_ok=True)
    (args.output_dir / "cce-release-manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    (args.output_dir / "release-workload.json").write_text(
        json.dumps(workload, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
