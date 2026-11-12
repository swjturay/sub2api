#!/usr/bin/env python3
"""Stage the checksum-pinned portable Python used by the Windows setup script."""

import hashlib
import json
from pathlib import Path
import re
import tempfile
import time
import urllib.error
import urllib.request


MAX_BYTES = 32 * 1024 * 1024


def stage(root: Path, open_url=urllib.request.urlopen) -> dict:
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
            temporary.unlink(missing_ok=True)


if __name__ == "__main__":
    for attempt in range(3):
        try:
            result = stage(Path(__file__).resolve().parents[2])
            print(json.dumps(result), flush=True)
            break
        except (urllib.error.URLError, TimeoutError):
            if attempt == 2:
                raise
            print("Retrying portable Python download", flush=True)
            time.sleep(2)
