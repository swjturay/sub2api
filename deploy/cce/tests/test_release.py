import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


MODULE_PATH = Path(__file__).resolve().parents[1] / "release.py"
SPEC = importlib.util.spec_from_file_location("cce_release", MODULE_PATH)
release = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(release)


def deployment(name: str, container: str) -> dict:
    return {
        "metadata": {
            "name": name,
            "generation": 1,
            "annotations": {"deployment.kubernetes.io/revision": "7"},
        },
        "spec": {
            "replicas": 1,
            "selector": {"matchLabels": {"app.kubernetes.io/name": name}},
            "template": {
                "metadata": {"labels": {"app.kubernetes.io/name": name}},
                "spec": {
                    "containers": [
                        {
                            "name": container,
                            "image": "old@example.invalid/image@sha256:" + "0" * 64,
                            "env": [{"name": "UNCHANGED", "value": "true"}],
                            "volumeMounts": [
                                {"name": "config", "mountPath": "/config"},
                                {"name": "previous-assets", "mountPath": "/previous"},
                            ],
                        }
                    ],
                    "initContainers": [{"name": "retain-previous-ui"}],
                    "imagePullSecrets": [{"name": "default-secret"}, {"name": "acr-pull"}],
                    "terminationGracePeriodSeconds": 30,
                    "volumes": [
                        {"name": "config", "configMap": {"name": "old-config"}},
                        {"name": "previous-assets", "emptyDir": {}},
                    ],
                },
            },
        },
    }


class ReleasePatchTest(unittest.TestCase):
    def test_backend_patch_enables_application_graceful_shutdown(self):
        source = deployment("sub2api", "sub2api")
        image = "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "1" * 64
        patch = release.build_backend_patch(source, image, {"release": "test"})
        pod_spec = patch["spec"]["template"]["spec"]
        container = release.named_container(pod_spec, "sub2api")

        self.assertEqual(container["image"], image)
        self.assertEqual(container["lifecycle"]["preStop"]["exec"]["command"][-1], "sleep 5")
        self.assertIn(
            {"name": "SERVER_SHUTDOWN_TIMEOUT_SECONDS", "value": "60"}, container["env"]
        )
        self.assertIn(
            {"name": "INSIGHTS_TRUSTED_COLLECTION", "value": "true"}, container["env"]
        )
        self.assertIn(
            {
                "name": "INSIGHTS_TRUSTED_USAGE_HISTORY_FROM",
                "value": "2026-06-01",
            },
            container["env"],
        )
        self.assertEqual(pod_spec["terminationGracePeriodSeconds"], 75)
        self.assertEqual(
            pod_spec["imagePullSecrets"], [{"name": "acr-pull"}, {"name": "default-secret"}]
        )

    def test_insights_patch_removes_all_previous_version_content(self):
        source = deployment("sub2api-insights", "insights")
        image = "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "2" * 64
        patch = release.build_insights_patch(
            source, image, {"release": "test"}, "sub2api-insights-nginx"
        )
        pod_spec = patch["spec"]["template"]["spec"]
        container = release.named_container(pod_spec, "insights")

        self.assertEqual(container["image"], image)
        self.assertIsNone(pod_spec["initContainers"])
        self.assertNotIn("previous-assets", {item["name"] for item in pod_spec["volumes"]})
        self.assertNotIn("previous-assets", {item["name"] for item in container["volumeMounts"]})
        config = next(item for item in pod_spec["volumes"] if item["name"] == "config")
        self.assertEqual(config["configMap"]["name"], "sub2api-insights-nginx")
        self.assertEqual(patch["spec"]["template"]["metadata"]["annotations"][
            "sub2api.2ray.wang/release-stage"
        ], "direct")

    def test_manifest_requires_both_immutable_acr_images(self):
        image = "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "3" * 64
        data = {
            "schema_version": "2",
            "source_ref": "codex/release",
            "source_sha": "a" * 40,
            "version": "0.2.8",
            "images": {
                "backend": {"immutable_image": image, "source_sha": "a" * 40},
                "insights": {"immutable_image": image, "source_sha": "a" * 40},
            },
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(data), encoding="utf-8")
            self.assertEqual(release.load_manifest(path)["source_sha"], "a" * 40)
            data["images"]["insights"]["immutable_image"] = "latest"
            path.write_text(json.dumps(data), encoding="utf-8")
            with self.assertRaises(release.ReleaseError):
                release.load_manifest(path)


if __name__ == "__main__":
    unittest.main()
