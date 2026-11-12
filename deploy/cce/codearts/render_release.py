#!/usr/bin/env python3
"""Render the immutable release manifest for the SRE operations host."""

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


def render(args: argparse.Namespace) -> dict:
    if not SHA_RE.fullmatch(args.source_sha):
        raise ValueError("source SHA must be a full lowercase commit SHA")
    if not VERSION_RE.fullmatch(args.version):
        raise ValueError("version is not a safe image tag component")
    if not RUN_RE.fullmatch(args.pipeline_run):
        raise ValueError("pipeline run identifier is unsafe")
    for name, image in (
        ("backend", args.backend_image),
        ("insights", args.insights_image),
    ):
        if not IMAGE_RE.fullmatch(image):
            raise ValueError(f"{name} image is not an immutable production ACR digest")

    created_at = dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat()
    return {
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


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    result.add_argument("--source-sha", required=True)
    result.add_argument("--version", required=True)
    result.add_argument("--pipeline-run", required=True)
    result.add_argument("--backend-image", required=True)
    result.add_argument("--insights-image", required=True)
    result.add_argument("--output-dir", type=Path, required=True)
    return result


def main() -> int:
    args = parser().parse_args()
    manifest = render(args)
    args.output_dir.mkdir(parents=True, exist_ok=True)
    (args.output_dir / "cce-release-manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
