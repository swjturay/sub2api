import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


CCE_DIR = Path(__file__).resolve().parents[1]


def load(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


build_bundle = load("build_bundle_for_host_test", CCE_DIR / "build_bundle.py")
host = load("host_release", CCE_DIR / "host_release.py")


def fixture(root: Path) -> dict:
    source = root / "source"
    release_dir = source / "release"
    release_dir.mkdir(parents=True)
    manifest = {
        "schema_version": "2",
        "source_ref": "cce-deploy",
        "source_sha": "a" * 40,
        "version": "0.2.21-cce-codearts-24",
        "images": {
            name: {
                "immutable_image": "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:"
                + digit * 64,
                "source_sha": "a" * 40,
            }
            for name, digit in (("backend", "1"), ("insights", "2"))
        },
    }
    (release_dir / "cce-release-manifest.json").write_text(
        json.dumps(manifest), encoding="utf-8"
    )
    release_tool = source / "release.py"
    release_tool.write_text(
        "import json\nprint(json.dumps({'phase': 'success', 'source_sha': '" + "a" * 40 + "'}))\n",
        encoding="utf-8",
    )
    return build_bundle.build(release_dir, release_tool, root / "artifact")


class HostReleaseTest(unittest.TestCase):
    def test_install_script_installs_release_and_host_entrypoints(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            release_destination = root / "sub2api-cce-release"
            host_destination = root / "sub2api-cce-host"
            env = {
                **os.environ,
                "SUB2API_CCE_RELEASE_BIN": str(release_destination),
                "SUB2API_CCE_HOST_BIN": str(host_destination),
            }

            subprocess.run(
                ["/bin/sh", str(CCE_DIR / "install.sh")],
                env=env,
                check=True,
                capture_output=True,
                text=True,
            )

            self.assertEqual(release_destination.read_bytes(), (CCE_DIR / "release.py").read_bytes())
            self.assertEqual(host_destination.read_bytes(), (CCE_DIR / "host_release.py").read_bytes())
            self.assertEqual(release_destination.stat().st_mode & 0o777, 0o755)
            self.assertEqual(host_destination.stat().st_mode & 0o777, 0o755)

    def test_run_persists_terminal_result_and_wait_reads_it(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = fixture(root)
            control_root = root / "control"
            host.stage_bundle(artifact["archive"], artifact["checksum"], control_root)
            host.start_release(
                control_root,
                artifact["release_id"],
                expected_current_sha="b" * 40,
                preflight_fn=lambda *_args, **_kwargs: {"phase": "preflight_passed"},
                run=lambda command: subprocess.CompletedProcess(command, 0, "accepted", ""),
            )

            result = host.run_release(
                control_root,
                artifact["release_id"],
                expected_current_sha="b" * 40,
            )
            waited = host.wait_release(
                control_root, artifact["release_id"], timeout=0, poll_interval=0
            )

            release_dir = control_root / "releases" / artifact["release_id"]
            self.assertEqual(result["phase"], "success")
            self.assertEqual(waited["phase"], "success")
            self.assertEqual(result["result"]["source_sha"], "a" * 40)
            self.assertIn('"phase": "success"', (release_dir / "stdout.log").read_text())
            self.assertEqual((release_dir / "stderr.log").read_text(), "")
            self.assertGreaterEqual(result["duration_seconds"], 0)

    def test_start_returns_after_systemd_accepts_the_durable_unit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = fixture(root)
            control_root = root / "control"
            host.stage_bundle(artifact["archive"], artifact["checksum"], control_root)
            commands = []

            def run(command):
                commands.append(command)
                return subprocess.CompletedProcess(command, 0, "Running as unit: test.service\n", "")

            result = host.start_release(
                control_root,
                artifact["release_id"],
                expected_current_sha="b" * 40,
                executable=Path("/usr/local/sbin/sub2api-cce-host"),
                preflight_fn=lambda *_args, **_kwargs: {"phase": "preflight_passed"},
                run=run,
                environment={"HOME": "/root"},
            )
            status = host.release_status(control_root, artifact["release_id"])

        self.assertEqual(result["phase"], "starting")
        self.assertEqual(status["phase"], "starting")
        self.assertEqual(result["unit"], "sub2api-release-0.2.21-cce-codearts-24-aaaaaaaaaaaa")
        self.assertEqual(commands[0][0], "systemd-run")
        self.assertIn("--setenv=HOME=/root", commands[0])
        self.assertIn("run", commands[0])
        self.assertIn("--expected-current-sha", commands[0])

    def test_failed_release_requires_explicit_retry_and_preserves_attempt(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = fixture(root)
            control_root = root / "control"
            staged = host.stage_bundle(artifact["archive"], artifact["checksum"], control_root)
            release_dir = control_root / "releases" / artifact["release_id"]
            host.atomic_json(
                release_dir / "state.json",
                {**staged, "phase": "failed", "error": "first failure", "returncode": 1},
            )

            with self.assertRaisesRegex(host.HostReleaseError, "explicit retry"):
                host.start_release(
                    control_root,
                    artifact["release_id"],
                    expected_current_sha="b" * 40,
                    preflight_fn=lambda *_args, **_kwargs: {"phase": "preflight_passed"},
                    run=lambda command: subprocess.CompletedProcess(command, 0, "accepted", ""),
                )
            result = host.start_release(
                control_root,
                artifact["release_id"],
                expected_current_sha="b" * 40,
                retry=True,
                preflight_fn=lambda *_args, **_kwargs: {"phase": "preflight_passed"},
                run=lambda command: subprocess.CompletedProcess(command, 0, "accepted", ""),
            )

            self.assertEqual(result["attempt"], 2)
            self.assertNotIn("error", result)
            self.assertNotIn("returncode", result)
            self.assertEqual(result["previous_attempts"][0]["phase"], "failed")
            self.assertEqual(result["previous_attempts"][0]["error"], "first failure")

    def test_preflight_reports_one_healthy_production_baseline(self):
        current_sha = "b" * 40
        deployments = {
            "items": [
                {
                    "metadata": {
                        "name": name,
                        "annotations": {"sub2api.2ray.wang/source-revision": current_sha},
                    },
                    "spec": {"replicas": replicas},
                    "status": {
                        "observedGeneration": 4,
                        "replicas": replicas,
                        "updatedReplicas": replicas,
                        "readyReplicas": replicas,
                        "availableReplicas": replicas,
                    },
                }
                for name, replicas in (("sub2api", 1), ("sub2api-insights", 2))
            ]
        }
        services = {
            "items": [
                {"metadata": {"name": name}, "spec": {"selector": {"app.kubernetes.io/name": name}}}
                for name in ("sub2api", "sub2api-insights")
            ]
        }

        def run(command):
            if command[:3] == ["kubectl", "config", "current-context"]:
                return subprocess.CompletedProcess(command, 0, "hispark-cce\n", "")
            if "deployment" in command:
                return subprocess.CompletedProcess(command, 0, json.dumps(deployments), "")
            if "service" in command:
                return subprocess.CompletedProcess(command, 0, json.dumps(services), "")
            if command[0] == "systemd-run":
                return subprocess.CompletedProcess(command, 0, "systemd 249\n", "")
            raise AssertionError(command)

        with tempfile.TemporaryDirectory() as directory:
            result = host.preflight(
                Path(directory),
                expected_current_sha=current_sha,
                expected_hostname="flexusx-hispark-sre",
                hostname=lambda: "flexusx-hispark-sre",
                run=run,
                disk_usage=lambda _: shutil._ntuple_diskusage(10_000_000_000, 1_000, 9_000_000_000),
                lock_available=lambda _: True,
            )

        self.assertEqual(result["phase"], "preflight_passed")
        self.assertEqual(result["current_source_sha"], current_sha)
        self.assertEqual(result["deployments"], {"sub2api": "1/1", "sub2api-insights": "2/2"})

    def test_preflight_rejects_a_pinned_service(self):
        current_sha = "b" * 40

        def run(command):
            if command[:3] == ["kubectl", "config", "current-context"]:
                return subprocess.CompletedProcess(command, 0, "hispark-cce\n", "")
            if "deployment" in command:
                payload = {
                    "items": [
                        {
                            "metadata": {"name": name, "annotations": {"sub2api.2ray.wang/source-revision": current_sha}},
                            "spec": {"replicas": replicas},
                            "status": {field: replicas for field in ("replicas", "updatedReplicas", "readyReplicas", "availableReplicas")},
                        }
                        for name, replicas in (("sub2api", 1), ("sub2api-insights", 2))
                    ]
                }
                return subprocess.CompletedProcess(command, 0, json.dumps(payload), "")
            if "service" in command:
                payload = {
                    "items": [
                        {"metadata": {"name": "sub2api"}, "spec": {"selector": {"pod-template-hash": "old"}}},
                        {"metadata": {"name": "sub2api-insights"}, "spec": {"selector": {"app": "insights"}}},
                    ]
                }
                return subprocess.CompletedProcess(command, 0, json.dumps(payload), "")
            if command[0] == "systemd-run":
                return subprocess.CompletedProcess(command, 0, "systemd 249\n", "")
            raise AssertionError(command)

        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(host.HostReleaseError, "pinned"):
                host.preflight(
                    Path(directory),
                    expected_current_sha=current_sha,
                    expected_hostname="flexusx-hispark-sre",
                    hostname=lambda: "flexusx-hispark-sre",
                    run=run,
                    disk_usage=lambda _: shutil._ntuple_diskusage(10_000_000_000, 1_000, 9_000_000_000),
                    lock_available=lambda _: True,
                )

    def test_stage_verifies_and_installs_exact_bundle(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = fixture(root)

            result = host.stage_bundle(
                artifact["archive"], artifact["checksum"], root / "control"
            )

            release_dir = root / "control" / "releases" / artifact["release_id"]
            self.assertEqual(result["phase"], "staged")
            self.assertEqual(result["release_id"], artifact["release_id"])
            self.assertEqual(json.loads((release_dir / "state.json").read_text())["phase"], "staged")
            self.assertEqual((release_dir / "release.py").stat().st_mode & 0o777, 0o555)
            self.assertEqual((release_dir / "cce-release-manifest.json").stat().st_mode & 0o777, 0o600)

    def test_stage_rejects_wrong_artifact_checksum(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = fixture(root)
            artifact["checksum"].write_text("0" * 64 + "  " + artifact["archive"].name + "\n")

            with self.assertRaisesRegex(host.HostReleaseError, "bundle SHA-256"):
                host.stage_bundle(artifact["archive"], artifact["checksum"], root / "control")


if __name__ == "__main__":
    unittest.main()
