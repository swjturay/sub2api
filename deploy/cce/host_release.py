#!/usr/bin/env python3
"""Durable SRE-host interface for staged Sub2API CCE releases."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import importlib.machinery
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import uuid

try:
    import fcntl
except ImportError:  # pragma: no cover - production host is Linux
    fcntl = None


MODULE_DIR = Path(__file__).resolve().parent


def load_release_module():
    candidate = MODULE_DIR / "release.py"
    if not candidate.is_file():
        candidate = Path(
            os.environ.get("SUB2API_CCE_RELEASE_BIN", "/usr/local/sbin/sub2api-cce-release")
        )
    loader = importlib.machinery.SourceFileLoader("sub2api_cce_release_contract", str(candidate))
    spec = importlib.util.spec_from_loader(loader.name, loader)
    if spec is None:
        raise RuntimeError(f"cannot load release contract from {candidate}")
    module = importlib.util.module_from_spec(spec)
    loader.exec_module(module)
    return module


release = load_release_module()


RELEASE_ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
EXPECTED_FILES = {"bundle.json", "cce-release-manifest.json", "release.py"}
MAX_BUNDLE_BYTES = 16 * 1024 * 1024
DEFAULT_ROOT = Path("/root/sub2api-release-control")
DEFAULT_RELEASE_LOCK = Path("/tmp/sub2api-cce-release.lock")
MINIMUM_FREE_BYTES = 2 * 1024 * 1024 * 1024


class HostReleaseError(RuntimeError):
    pass


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def atomic_json(path: Path, value: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    temporary = path.with_name(f".{path.name}.{uuid.uuid4().hex}.tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    temporary.chmod(0o600)
    os.replace(temporary, path)


def checked_run(command: list[str]) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(command, text=True, capture_output=True, check=False)
    if result.returncode:
        raise HostReleaseError(
            f"command failed ({result.returncode}): {' '.join(command)}: {result.stderr.strip()}"
        )
    return result


def release_lock_available(path: Path) -> bool:
    if fcntl is None:
        raise HostReleaseError("release lock inspection requires fcntl")
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a+", encoding="utf-8") as stream:
        try:
            fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return False
        fcntl.flock(stream, fcntl.LOCK_UN)
    return True


def preflight(
    control_root: Path = DEFAULT_ROOT,
    *,
    expected_current_sha: str = "",
    expected_hostname: str = "flexusx-hispark-sre",
    hostname=os.uname,
    run=checked_run,
    disk_usage=shutil.disk_usage,
    lock_available=release_lock_available,
    release_lock: Path = DEFAULT_RELEASE_LOCK,
    minimum_free_bytes: int = MINIMUM_FREE_BYTES,
) -> dict:
    host_value = hostname()
    if not isinstance(host_value, str):
        host_value = host_value.nodename
    if host_value != expected_hostname:
        raise HostReleaseError(f"unexpected operations host: {host_value!r}")
    run(["systemd-run", "--version"])
    context = run(["kubectl", "config", "current-context"]).stdout.strip()
    if context != "hispark-cce":
        raise HostReleaseError(f"unexpected kubectl context: {context!r}")
    prefix = ["kubectl", "--context", "hispark-cce", "-n", "sub2api-prod", "get"]
    deployments = json.loads(
        run([*prefix, "deployment", "sub2api", "sub2api-insights", "-o", "json"]).stdout
    )
    deployment_summary = {}
    source_shas = set()
    for item in deployments.get("items", []):
        name = item.get("metadata", {}).get("name")
        desired = int(item.get("spec", {}).get("replicas", 0))
        status = item.get("status", {})
        for field in ("replicas", "updatedReplicas", "readyReplicas", "availableReplicas"):
            if int(status.get(field, 0)) != desired:
                raise HostReleaseError(f"deployment {name} {field} is not {desired}")
        source_sha = item.get("metadata", {}).get("annotations", {}).get(
            "sub2api.2ray.wang/source-revision"
        )
        if not release.SHA_RE.fullmatch(str(source_sha)):
            raise HostReleaseError(f"deployment {name} has no valid source revision")
        source_shas.add(source_sha)
        deployment_summary[str(name)] = f"{desired}/{desired}"
    if set(deployment_summary) != {"sub2api", "sub2api-insights"}:
        raise HostReleaseError("production deployment inventory is incomplete")
    if len(source_shas) != 1:
        raise HostReleaseError("production deployments do not share one source revision")
    current_sha = next(iter(source_shas))
    if expected_current_sha and current_sha != expected_current_sha:
        raise HostReleaseError(
            f"production source changed: {current_sha!r} != {expected_current_sha!r}"
        )
    services = json.loads(
        run([*prefix, "service", "sub2api", "sub2api-insights", "-o", "json"]).stdout
    )
    for item in services.get("items", []):
        name = item.get("metadata", {}).get("name")
        selector = item.get("spec", {}).get("selector", {})
        if "pod-template-hash" in selector:
            raise HostReleaseError(f"service {name} is pinned to a rollout hash")
    if len(services.get("items", [])) != 2:
        raise HostReleaseError("production service inventory is incomplete")
    if not lock_available(release_lock):
        raise HostReleaseError("another CCE release holds the operations-host lock")
    disk_root = control_root if control_root.exists() else control_root.parent
    free_bytes = disk_usage(disk_root).free
    if free_bytes < minimum_free_bytes:
        raise HostReleaseError(
            f"operations-host free disk is below {minimum_free_bytes} bytes"
        )
    return {
        "schema_version": 1,
        "phase": "preflight_passed",
        "at": utc_now(),
        "hostname": host_value,
        "context": context,
        "namespace": "sub2api-prod",
        "current_source_sha": current_sha,
        "deployments": deployment_summary,
        "free_bytes": free_bytes,
        "release_lock_available": True,
    }


def read_checksum(checksum_path: Path, archive_name: str) -> str:
    fields = checksum_path.read_text(encoding="utf-8").strip().split()
    if len(fields) != 2 or not SHA256_RE.fullmatch(fields[0]) or fields[1] != archive_name:
        raise HostReleaseError("bundle checksum sidecar is invalid")
    return fields[0]


def read_bundle(archive_path: Path, checksum_path: Path) -> tuple[dict, dict[str, bytes], str]:
    archive_path = archive_path.resolve()
    checksum_path = checksum_path.resolve()
    if archive_path.stat().st_size > MAX_BUNDLE_BYTES:
        raise HostReleaseError("bundle exceeds size limit")
    archive_payload = archive_path.read_bytes()
    expected = read_checksum(checksum_path, archive_path.name)
    actual = sha256_bytes(archive_payload)
    if actual != expected:
        raise HostReleaseError("bundle SHA-256 does not match sidecar")
    with tarfile.open(archive_path, "r:gz") as archive:
        members = archive.getmembers()
        names = [member.name for member in members]
        if set(names) != EXPECTED_FILES or len(names) != len(EXPECTED_FILES):
            raise HostReleaseError("bundle member list is invalid")
        if any(not member.isfile() or member.size > MAX_BUNDLE_BYTES for member in members):
            raise HostReleaseError("bundle contains an unsafe member")
        files = {}
        for member in members:
            stream = archive.extractfile(member)
            if stream is None:
                raise HostReleaseError(f"bundle member {member.name} cannot be read")
            files[member.name] = stream.read()
    metadata = json.loads(files["bundle.json"])
    if metadata.get("schema_version") != 1:
        raise HostReleaseError("bundle schema_version must be 1")
    release_id = str(metadata.get("release_id", ""))
    if not RELEASE_ID_RE.fullmatch(release_id):
        raise HostReleaseError("bundle release_id is unsafe")
    declared = metadata.get("files")
    if not isinstance(declared, dict) or set(declared) != EXPECTED_FILES - {"bundle.json"}:
        raise HostReleaseError("bundle file checksum contract is invalid")
    for name, digest in declared.items():
        if not SHA256_RE.fullmatch(str(digest)) or sha256_bytes(files[name]) != digest:
            raise HostReleaseError(f"bundle member {name} SHA-256 does not match")
    return metadata, files, actual


def stage_bundle(archive_path: Path, checksum_path: Path, control_root: Path = DEFAULT_ROOT) -> dict:
    metadata, files, bundle_sha = read_bundle(archive_path, checksum_path)
    control_root = control_root.resolve()
    releases_root = control_root / "releases"
    releases_root.mkdir(parents=True, exist_ok=True, mode=0o700)
    releases_root.chmod(0o700)
    release_id = metadata["release_id"]
    destination = releases_root / release_id
    if destination.exists():
        state = json.loads((destination / "state.json").read_text(encoding="utf-8"))
        if state.get("bundle_sha256") != bundle_sha:
            raise HostReleaseError(f"release_id {release_id} is already staged with different bytes")
        return state
    temporary = Path(tempfile.mkdtemp(prefix=f".{release_id}.", dir=releases_root))
    try:
        temporary.chmod(0o700)
        for name in ("cce-release-manifest.json", "release.py", "bundle.json"):
            path = temporary / name
            path.write_bytes(files[name])
            path.chmod(0o555 if name == "release.py" else 0o600)
        manifest = release.load_manifest(temporary / "cce-release-manifest.json")
        if manifest["source_sha"] != metadata.get("source_sha"):
            raise HostReleaseError("bundle source SHA does not match release manifest")
        if manifest["version"] != metadata.get("version"):
            raise HostReleaseError("bundle version does not match release manifest")
        state = {
            "schema_version": 1,
            "phase": "staged",
            "release_id": release_id,
            "source_sha": manifest["source_sha"],
            "version": manifest["version"],
            "bundle_sha256": bundle_sha,
            "staged_at": utc_now(),
        }
        atomic_json(temporary / "state.json", state)
        os.replace(temporary, destination)
        return state
    except Exception:
        shutil.rmtree(temporary, ignore_errors=True)
        raise


def release_directory(control_root: Path, release_id: str) -> Path:
    if not RELEASE_ID_RE.fullmatch(release_id):
        raise HostReleaseError("release_id is unsafe")
    return control_root.resolve() / "releases" / release_id


def release_status(control_root: Path, release_id: str) -> dict:
    state_path = release_directory(control_root, release_id) / "state.json"
    if not state_path.is_file():
        raise HostReleaseError(f"release_id {release_id} is not staged")
    value = json.loads(state_path.read_text(encoding="utf-8"))
    if value.get("release_id") != release_id:
        raise HostReleaseError("release state identity does not match its directory")
    return value


def unit_name(release_id: str) -> str:
    value = f"sub2api-release-{release_id}"
    if len(value) > 240:
        value = value[:220] + "-" + hashlib.sha256(value.encode()).hexdigest()[:16]
    return value


def start_release(
    control_root: Path,
    release_id: str,
    *,
    expected_current_sha: str,
    executable: Path = Path("/usr/local/sbin/sub2api-cce-host"),
    preflight_fn=preflight,
    run=checked_run,
    environment=os.environ,
    retry: bool = False,
) -> dict:
    if not release.SHA_RE.fullmatch(expected_current_sha):
        raise HostReleaseError("expected current source must be a full SHA")
    state = release_status(control_root, release_id)
    if state.get("phase") == "failed" and not retry:
        raise HostReleaseError(f"release {release_id} requires an explicit retry")
    if state.get("phase") not in {"staged", "failed_to_start"}:
        if not (retry and state.get("phase") == "failed"):
            raise HostReleaseError(
                f"release {release_id} cannot start from phase {state.get('phase')!r}"
            )
    report = preflight_fn(control_root, expected_current_sha=expected_current_sha)
    unit = unit_name(release_id)
    previous_attempts = list(state.get("previous_attempts", []))
    if state.get("phase") == "failed":
        previous_attempts.append(
            {
                key: state.get(key)
                for key in ("phase", "error", "finished_at", "duration_seconds", "returncode")
                if state.get(key) is not None
            }
        )
        old_attempt = int(state.get("attempt", 1))
        for name in ("stdout.log", "stderr.log"):
            path = release_directory(control_root, release_id) / name
            if path.exists():
                path.replace(path.with_name(f"{path.stem}.attempt-{old_attempt}{path.suffix}"))
    attempt = int(state.get("attempt", 0)) + 1
    starting = {
        **state,
        "phase": "starting",
        "attempt": attempt,
        "previous_attempts": previous_attempts,
        "unit": unit,
        "expected_current_sha": expected_current_sha,
        "start_requested_at": utc_now(),
        "preflight": report,
    }
    state_path = release_directory(control_root, release_id) / "state.json"
    atomic_json(state_path, starting)
    command = [
        "systemd-run",
        f"--unit={unit}",
        "--collect",
        "--property=Type=exec",
        "--property=TimeoutStartSec=infinity",
        "--setenv=PYTHONUNBUFFERED=1",
        f"--setenv=HOME={environment.get('HOME', '/root')}",
        str(executable),
        "--control-root",
        str(control_root.resolve()),
        "run",
        "--release-id",
        release_id,
        "--expected-current-sha",
        expected_current_sha,
    ]
    try:
        run(command)
    except Exception as error:
        failed = {
            **starting,
            "phase": "failed_to_start",
            "finished_at": utc_now(),
            "error": str(error),
        }
        atomic_json(state_path, failed)
        raise
    return starting


def terminal_result(stdout: str) -> dict:
    for line in reversed(stdout.splitlines()):
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict):
            return value
    raise HostReleaseError("release command produced no JSON result")


def run_release(
    control_root: Path,
    release_id: str,
    *,
    expected_current_sha: str,
    python: str = sys.executable,
) -> dict:
    release_dir = release_directory(control_root, release_id)
    state_path = release_dir / "state.json"
    state = release_status(control_root, release_id)
    if state.get("phase") != "starting":
        raise HostReleaseError(
            f"release {release_id} cannot run from phase {state.get('phase')!r}"
        )
    if state.get("expected_current_sha") != expected_current_sha:
        raise HostReleaseError("run expected SHA does not match staged start request")
    started = time.monotonic()
    running = {**state, "phase": "running", "started_at": utc_now()}
    atomic_json(state_path, running)
    command = [
        python,
        str(release_dir / "release.py"),
        "--manifest",
        str(release_dir / "cce-release-manifest.json"),
        "--expected-current-sha",
        expected_current_sha,
        "--audit-root",
        str(release_dir / "audit"),
    ]
    result = subprocess.run(command, text=True, capture_output=True, check=False)
    stdout_path = release_dir / "stdout.log"
    stderr_path = release_dir / "stderr.log"
    stdout_path.write_text(result.stdout, encoding="utf-8")
    stderr_path.write_text(result.stderr, encoding="utf-8")
    stdout_path.chmod(0o600)
    stderr_path.chmod(0o600)
    duration = round(time.monotonic() - started, 3)
    try:
        release_result = terminal_result(result.stdout)
    except HostReleaseError:
        release_result = None
    if result.returncode == 0 and release_result and release_result.get("phase") == "success":
        final = {
            **running,
            "phase": "success",
            "finished_at": utc_now(),
            "duration_seconds": duration,
            "returncode": result.returncode,
            "result": release_result,
        }
        atomic_json(state_path, final)
        return final
    final = {
        **running,
        "phase": "failed",
        "finished_at": utc_now(),
        "duration_seconds": duration,
        "returncode": result.returncode,
        "result": release_result,
        "error": result.stderr.strip() or "release command failed without stderr",
    }
    atomic_json(state_path, final)
    raise HostReleaseError(final["error"])


def wait_release(
    control_root: Path,
    release_id: str,
    *,
    timeout: int,
    poll_interval: float = 2,
) -> dict:
    if timeout < 0:
        raise HostReleaseError("wait timeout cannot be negative")
    deadline = time.monotonic() + timeout
    while True:
        state = release_status(control_root, release_id)
        if state.get("phase") in {"success", "failed", "failed_to_start"}:
            return state
        if time.monotonic() >= deadline:
            raise HostReleaseError(
                f"release {release_id} is still {state.get('phase')!r} after {timeout}s"
            )
        time.sleep(min(poll_interval, max(0, deadline - time.monotonic())))


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    result.add_argument("--control-root", type=Path, default=DEFAULT_ROOT)
    commands = result.add_subparsers(dest="command", required=True)
    check = commands.add_parser("preflight", help="check the production release path without mutation")
    check.add_argument("--expected-current-sha", default="")
    check.add_argument("--expected-hostname", default="flexusx-hispark-sre")
    stage = commands.add_parser("stage", help="verify and stage one immutable release bundle")
    stage.add_argument("--bundle", type=Path, required=True)
    stage.add_argument("--checksum", type=Path, required=True)
    start = commands.add_parser("start", help="start one staged release as a durable systemd unit")
    start.add_argument("--release-id", required=True)
    start.add_argument("--expected-current-sha", required=True)
    start.add_argument("--retry", action="store_true")
    status = commands.add_parser("status", help="read the persistent state of one release")
    status.add_argument("--release-id", required=True)
    wait = commands.add_parser("wait", help="wait for a persistent terminal release state")
    wait.add_argument("--release-id", required=True)
    wait.add_argument("--timeout", type=int, default=900)
    run_command = commands.add_parser("run", help=argparse.SUPPRESS)
    run_command.add_argument("--release-id", required=True)
    run_command.add_argument("--expected-current-sha", required=True)
    return result


def main() -> int:
    args = parser().parse_args()
    if args.command == "preflight":
        value = preflight(
            args.control_root,
            expected_current_sha=args.expected_current_sha,
            expected_hostname=args.expected_hostname,
        )
    elif args.command == "stage":
        value = stage_bundle(args.bundle, args.checksum, args.control_root)
    elif args.command == "start":
        value = start_release(
            args.control_root,
            args.release_id,
            expected_current_sha=args.expected_current_sha,
            retry=args.retry,
        )
    elif args.command == "status":
        value = release_status(args.control_root, args.release_id)
    elif args.command == "wait":
        value = wait_release(args.control_root, args.release_id, timeout=args.timeout)
    elif args.command == "run":
        value = run_release(
            args.control_root,
            args.release_id,
            expected_current_sha=args.expected_current_sha,
        )
    else:  # pragma: no cover - argparse rejects unknown commands
        raise HostReleaseError(f"unsupported command: {args.command}")
    print(json.dumps(value, sort_keys=True), flush=True)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (HostReleaseError, OSError, ValueError, json.JSONDecodeError, tarfile.TarError) as error:
        print(f"host release failed: {error}", file=sys.stderr)
        raise SystemExit(1)
