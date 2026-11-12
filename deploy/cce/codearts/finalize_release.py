#!/usr/bin/env python3
"""Create the fixed operations-host bundle from non-secret immutable image inputs."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys


CODEARTS_DIR = Path(__file__).resolve().parent
CCE_DIR = CODEARTS_DIR.parent
for value in (CODEARTS_DIR, CCE_DIR):
    if str(value) not in sys.path:
        sys.path.insert(0, str(value))

import build_bundle  # noqa: E402
import render_release  # noqa: E402


EXPECTED_KEYS = {
    "source_sha",
    "version",
    "pipeline_run",
    "backend_image",
    "insights_image",
}


def finalize(value: dict, output_dir: Path, release_tool: Path) -> dict:
    if set(value) != EXPECTED_KEYS:
        raise ValueError(f"release input must contain exactly {sorted(EXPECTED_KEYS)}")
    args = argparse.Namespace(**value)
    manifest = render_release.render(args)
    output_dir.mkdir(parents=True, exist_ok=True)
    manifest_path = output_dir / "cce-release-manifest.json"
    manifest_path.write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    return build_bundle.build(
        output_dir,
        release_tool,
        output_dir,
        archive_name="sub2api-release-bundle.tar.gz",
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("release"))
    parser.add_argument("--release-tool", type=Path, default=CCE_DIR / "release.py")
    args = parser.parse_args()
    value = json.loads(args.input.read_text(encoding="utf-8"))
    result = finalize(value, args.output_dir, args.release_tool)
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
