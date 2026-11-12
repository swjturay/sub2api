#!/usr/bin/env python3
"""Build the immutable artifact consumed by the SRE operations host."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import json
from pathlib import Path
import re
import sys
import tarfile
from typing import Optional


MODULE_DIR = Path(__file__).resolve().parent
if str(MODULE_DIR) not in sys.path:
    sys.path.insert(0, str(MODULE_DIR))

import release  # noqa: E402


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def json_bytes(value: object) -> bytes:
    return (json.dumps(value, indent=2, sort_keys=True) + "\n").encode("utf-8")


def tar_bytes(files: list[tuple[str, bytes, int]]) -> bytes:
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode="w", format=tarfile.PAX_FORMAT) as archive:
        for name, payload, mode in files:
            info = tarfile.TarInfo(name)
            info.size = len(payload)
            info.mode = mode
            info.mtime = 0
            info.uid = 0
            info.gid = 0
            info.uname = ""
            info.gname = ""
            archive.addfile(info, io.BytesIO(payload))
    compressed = io.BytesIO()
    with gzip.GzipFile(fileobj=compressed, mode="wb", filename="", mtime=0) as stream:
        stream.write(raw.getvalue())
    return compressed.getvalue()


ARCHIVE_NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,159}\.tar\.gz$")


def build(
    release_dir: Path,
    release_tool: Path,
    output_dir: Path,
    archive_name: Optional[str] = None,
) -> dict:
    release_dir = release_dir.resolve()
    release_tool = release_tool.resolve()
    manifest_path = release_dir / "cce-release-manifest.json"
    manifest = release.load_manifest(manifest_path)
    manifest_payload = manifest_path.read_bytes()
    tool_payload = release_tool.read_bytes()
    release_id = f"{manifest['version']}-{manifest['source_sha'][:12]}"
    metadata = {
        "schema_version": 1,
        "release_id": release_id,
        "source_sha": manifest["source_sha"],
        "source_ref": manifest["source_ref"],
        "version": manifest["version"],
        "created_at": manifest.get("created_at"),
        "files": {
            "cce-release-manifest.json": sha256_bytes(manifest_payload),
            "release.py": sha256_bytes(tool_payload),
        },
    }
    payload = tar_bytes(
        [
            ("bundle.json", json_bytes(metadata), 0o600),
            ("cce-release-manifest.json", manifest_payload, 0o600),
            ("release.py", tool_payload, 0o555),
        ]
    )
    output_dir.mkdir(parents=True, exist_ok=True)
    archive_name = archive_name or f"sub2api-release-{release_id}.tar.gz"
    if not ARCHIVE_NAME_RE.fullmatch(archive_name):
        raise ValueError("archive name is unsafe")
    archive_path = output_dir / archive_name
    archive_path.write_bytes(payload)
    archive_path.chmod(0o600)
    digest = sha256_bytes(payload)
    checksum_path = archive_path.with_suffix(archive_path.suffix + ".sha256")
    checksum_path.write_text(f"{digest}  {archive_path.name}\n", encoding="utf-8")
    checksum_path.chmod(0o600)
    return {
        "release_id": release_id,
        "archive": archive_path,
        "checksum": checksum_path,
        "sha256": digest,
    }


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    result.add_argument("--release-dir", type=Path, required=True)
    result.add_argument("--release-tool", type=Path, default=MODULE_DIR / "release.py")
    result.add_argument("--output-dir", type=Path, required=True)
    result.add_argument("--archive-name")
    return result


def main() -> int:
    args = parser().parse_args()
    result = build(
        args.release_dir,
        args.release_tool,
        args.output_dir,
        archive_name=args.archive_name,
    )
    print(
        json.dumps(
            {
                **result,
                "archive": str(result["archive"]),
                "checksum": str(result["checksum"]),
            },
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
