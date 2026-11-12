import argparse
import importlib.util
import json
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


def load(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


render_release = load("render_release", ROOT / "render_release.py")
pipeline_config = load("pipeline_config", ROOT / "pipeline_config.py")
build_jobs = load("build_jobs", ROOT / "build_jobs.py")


class OperationsHostReleaseTest(unittest.TestCase):
    def test_build_task_specs_have_one_responsibility_and_fixed_bundle_outputs(self):
        specs = build_jobs.task_specs()

        self.assertEqual(set(specs), {
            "validate-release", "validate-frontend", "validate-insights", "validate-backend",
            "build-backend", "build-insights", "finalize",
        })
        for key, body in specs.items():
            names = [step["name"] for step in body["steps"]]
            self.assertEqual(names[0], "锁定精确cce-deploy源码")
            self.assertNotIn("runner", json.dumps(body).lower())
            parameters = {
                item["params"][0]["value"]
                for item in body["parameters"]
                if item["params"][0]["name"] == "name"
            }
            self.assertIn("SOURCE_SHA", parameters)
            self.assertIn("RELEASE_VERSION", parameters)
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
            "finalize": "7" * 32,
        }

        definition = pipeline_config.pipeline_definition(job_ids)

        self.assertEqual(len(definition["stages"]), 2)
        parallel = definition["stages"][0]["jobs"]
        self.assertEqual(len(parallel), 6)
        self.assertEqual({job["name"] for job in parallel}, {
            "发布工具验证", "原站前端验证", "Insights验证", "Backend验证", "Backend镜像", "Insights镜像"
        })
        self.assertTrue(all(job["depends_on"] == [] for job in parallel))
        for job in parallel:
            inputs = {item["key"]: item["value"] for item in job["steps"][0]["inputs"]}
            self.assertEqual(inputs["SOURCE_SHA"], "${SOURCE_SHA}")
            self.assertEqual(inputs["RELEASE_VERSION"], "${PIPELINE_NUMBER}")
        final = definition["stages"][1]["jobs"]
        self.assertEqual(len(final), 1)
        self.assertEqual(final[0]["name"], "生成固定发布制品")
        final_inputs = {item["key"]: item["value"] for item in final[0]["steps"][0]["inputs"]}
        self.assertEqual(final_inputs["ACR_USERNAME"], "${ACR_USERNAME}")
        self.assertEqual(final_inputs["ACR_PASSWORD"], "${ACR_PASSWORD}")
        self.assertTrue(all("ACR_PASSWORD" not in json.dumps(job) for job in parallel))
        self.assertNotIn("runner", json.dumps(definition).lower())

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
