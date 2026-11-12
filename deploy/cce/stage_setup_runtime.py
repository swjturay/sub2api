#!/usr/bin/env python3
"""Stage the checksum-pinned portable Python used by the Windows setup script."""

import hashlib
from contextlib import contextmanager
import json
from pathlib import Path
import re
import tempfile
import subprocess


MAX_BYTES = 32 * 1024 * 1024


@contextmanager
def open_runtime(url, timeout=60):
    official = "https://www.python.org/ftp/python/"
    if not url.startswith(official):
        raise ValueError("portable Python URL is not approved")
    mirror = url.replace(official, "https://repo.huaweicloud.com/python/", 1)
    with tempfile.NamedTemporaryFile(delete=False) as output:
        temporary = Path(output.name)
    try:
        for candidate in (mirror, url):
            print("Fetching portable Python from " + candidate.split('/')[2], flush=True)
            try:
                subprocess.run(
                    ["curl", "--fail", "--location", "--silent", "--show-error",
                     "--connect-timeout", "15", "--max-time", str(timeout),
                     "--max-filesize", str(MAX_BYTES), "--output", str(temporary), candidate],
                    check=True, capture_output=True, timeout=timeout + 15,
                )
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
                continue
            with temporary.open("rb") as response:
                yield response
            return
        raise RuntimeError("portable Python download failed for both approved sources")
    finally:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def stage(root: Path, open_url=open_runtime) -> dict:
    scripts = root / "frontend/public/scripts"
    bootstrap = (scripts / "sub2api-local-setup.ps1").read_text(encoding="utf-8-sig")

    def value(name):
        match = re.search(r"^\$" + name + r"\s*=\s*'([^']+)'\s*$", bootstrap, re.MULTILINE)
        if match is None:
            raise ValueError("portable Python bootstrap metadata is missing")
        return match.group(1)

    version, asset, checksum = value("pythonVersion"), value("pythonAsset"), value("pythonSha256")
    if not re.fullmatch(r"\d+\.\d+\.\d+", version):
        raise ValueError("portable Python version is invalid")
    if asset != f"python-{version}-embed-amd64.zip" or not re.fullmatch(r"[0-9a-f]{64}", checksum):
        raise ValueError("portable Python asset or checksum is invalid")
    destination = scripts / asset
    if (destination.exists() and destination.stat().st_size <= MAX_BYTES
            and hashlib.sha256(destination.read_bytes()).hexdigest() == checksum):
        return {"asset": asset, "sha256": checksum, "bytes": destination.stat().st_size, "cached": True}

    url = f"https://www.python.org/ftp/python/{version}/{asset}"
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=scripts, suffix=".part", delete=False) as output:
            temporary = Path(output.name)
            digest, size = hashlib.sha256(), 0
            with open_url(url, timeout=60) as response:
                for block in iter(lambda: response.read(64 * 1024), b""):
                    size += len(block)
                    if size > MAX_BYTES:
                        raise ValueError("portable Python exceeds the size limit")
                    digest.update(block)
                    output.write(block)
        if digest.hexdigest() != checksum:
            raise ValueError("portable Python SHA-256 mismatch")
        temporary.chmod(0o644)
        temporary.replace(destination)
        return {"asset": asset, "sha256": checksum, "bytes": size, "cached": False}
    finally:
        if temporary is not None:
            try:
                temporary.unlink()
            except FileNotFoundError:
                pass


if __name__ == "__main__":
    print(json.dumps(stage(Path(__file__).resolve().parents[2])), flush=True)
