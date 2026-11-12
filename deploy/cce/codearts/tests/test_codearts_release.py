import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
CCE_DIR = ROOT.parent


def load(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


render_release = load("render_release", ROOT / "render_release.py")
pipeline_config = load("pipeline_config", ROOT / "pipeline_config.py")
build_jobs = load("build_jobs", ROOT / "build_jobs.py")
finalize_release = load("finalize_release", ROOT / "finalize_release.py")


class OperationsHostReleaseTest(unittest.TestCase):
    def test_source_pin_accepts_exact_sha_and_rejects_other_shapes(self):
        validation = build_jobs.FETCH_SOURCE.split("rm -rf", 1)[0]
        base = {**os.environ, "RELEASE_VERSION": "24"}

        accepted = subprocess.run(
            ["/bin/sh", "-c", validation],
            env={**base, "SOURCE_SHA": "a" * 40},
            capture_output=True,
            text=True,
        )
        rejected = subprocess.run(
            ["/bin/sh", "-c", validation],
            env={**base, "SOURCE_SHA": "a" * 39 + "G"},
            capture_output=True,
            text=True,
        )

        self.assertEqual(accepted.returncode, 0)
        self.assertNotEqual(rejected.returncode, 0)
        self.assertIn("full lowercase commit", rejected.stderr)
        self.assertNotIn("\\\n", build_jobs.FETCH_SOURCE)

    def test_build_task_specs_have_one_responsibility_and_fixed_bundle_outputs(self):
        specs = build_jobs.task_specs("a" * 40, "29")

        self.assertEqual(set(specs), {
            "validate-release", "validate-frontend", "validate-insights", "validate-backend",
            "build-backend", "build-insights", "finalize",
        })
        for key, body in specs.items():
            names = [step["name"] for step in body["steps"]]
            self.assertEqual(names[0], "锁定精确cce-deploy源码")
            self.assertNotIn("runner", json.dumps(body).lower())
            self.assertIn("SOURCE_SHA=" + "a" * 40, body["steps"][0]["properties"]["command"])
            self.assertIn("RELEASE_VERSION=29", body["steps"][0]["properties"]["command"])
        final_parameters = {
            item["params"][0]["value"] for item in specs["finalize"]["parameters"]
        }
        self.assertEqual(final_parameters, {"RELEASE_INPUT_B64"})
        self.assertNotIn("ACR_", json.dumps(specs["finalize"]))
        uploads = [step["properties"]["file"] for step in specs["finalize"]["steps"] if step["name"].startswith("上传")]
        self.assertEqual(uploads, [
            "sub2api-source/release/sub2api-release-bundle.tar.gz",
            "sub2api-source/release/sub2api-release-bundle.tar.gz.sha256",
            "sub2api-source/release/cce-release-manifest.json",
        ])

    def test_pipeline_runs_validation_and_images_in_parallel_before_bundle(self):
        job_ids = {
            "validate-release": "1" * 32,
            "validate-frontend": "2" * 32,
            "validate-insights": "3" * 32,
            "validate-backend": "4" * 32,
            "build-backend": "5" * 32,
            "build-insights": "6" * 32,
        }

        definition = pipeline_config.pipeline_definition(job_ids)

        self.assertEqual(len(definition["stages"]), 1)
        parallel = definition["stages"][0]["jobs"]
        self.assertEqual(len(parallel), 6)
        self.assertEqual({job["name"] for job in parallel}, {
            "发布工具验证", "原站前端验证", "Insights验证", "Backend验证", "Backend镜像", "Insights镜像"
        })
        self.assertTrue(all(job["depends_on"] == [] for job in parallel))
        self.assertTrue(all("ACR_" not in json.dumps(job) for job in parallel))
        self.assertNotIn("runner", json.dumps(definition).lower())

    def test_finalizer_builds_bundle_from_non_secret_immutable_inputs(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "release"
            image = "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "a" * 64
            value = {
                "source_sha": "b" * 40,
                "version": "0.2.21-cce-codearts-29",
                "pipeline_run": "29",
                "backend_image": image,
                "insights_image": image,
            }

            result = finalize_release.finalize(value, output, CCE_DIR / "release.py")

            self.assertEqual(result["release_id"], "0.2.21-cce-codearts-29-bbbbbbbbbbbb")
            self.assertTrue((output / "sub2api-release-bundle.tar.gz").is_file())
            manifest = json.loads((output / "cce-release-manifest.json").read_text())
            self.assertEqual(set(manifest["images"]), {"backend", "insights"})

    def test_validation_contract_is_split_into_independent_images(self):
        expected = {
            "Dockerfile.validate.release": "python3 -m unittest discover -s deploy/cce/tests",
            "Dockerfile.validate.frontend": "pnpm --dir frontend run lint:check",
            "Dockerfile.validate.insights": "pnpm test",
            "Dockerfile.validate.backend": "go test -tags=unit",
        }

        for name, command in expected.items():
            source = (ROOT / name).read_text(encoding="utf-8")
            self.assertIn(command, source)
        self.assertNotIn("frontend", (ROOT / "Dockerfile.validate.backend").read_text())
        self.assertNotIn("backend", (ROOT / "Dockerfile.validate.insights").read_text())
        self.assertNotIn("insights", (ROOT / "Dockerfile.validate.release").read_text())

    def test_rendered_host_manifest_has_no_in_cluster_runner_contract(self):
        image = "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "a" * 64
        args = argparse.Namespace(
            source_sha="b" * 40,
            version="0.2.21-cce-codearts-24",
            pipeline_run="24",
            backend_image=image,
            insights_image=image,
        )

        manifest = render_release.render(args)

        self.assertEqual(manifest["source_sha"], "b" * 40)
        self.assertEqual(set(manifest["images"]), {"backend", "insights"})
        self.assertNotIn("runner", json.dumps(manifest))

    def test_renderer_rejects_mutable_images(self):
        args = argparse.Namespace(
            source_sha="b" * 40,
            version="0.2.21-cce-codearts-24",
            pipeline_run="24",
            backend_image="latest",
            insights_image="registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "a" * 64,
        )

        with self.assertRaisesRegex(ValueError, "backend image"):
            render_release.render(args)


if __name__ == "__main__":
    unittest.main()
