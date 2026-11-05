import argparse
import importlib.util
import json
from pathlib import Path
import sys
import unittest


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT.parent))


def load(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


slot = load("slot_release", ROOT / "slot_release.py")
render_release = load("render_release", ROOT / "render_release.py")


def source_deployment(name: str, container: str) -> dict:
    return {
        "apiVersion": "apps/v1",
        "metadata": {"name": name, "namespace": "sub2api-prod", "labels": {"app": name}},
        "spec": {
            "replicas": 1,
            "selector": {"matchLabels": {"app.kubernetes.io/name": name}},
            "template": {
                "metadata": {"labels": {"app.kubernetes.io/name": name}},
                "spec": {
                    "containers": [
                        {
                            "name": container,
                            "image": "old.invalid/image",
                            "env": [],
                            "volumeMounts": [{"name": "config", "mountPath": "/config"}],
                        }
                    ],
                    "volumes": [{"name": "config", "configMap": {"name": "config"}}],
                },
            },
        },
    }


class SlotReleaseTest(unittest.TestCase):
    def test_slot_selection(self):
        self.assertEqual(slot.other_slot(None), "blue")
        self.assertEqual(slot.other_slot("blue"), "green")
        self.assertEqual(slot.other_slot("green"), "blue")

    def test_candidate_selector_isolated_from_legacy_service(self):
        image = "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "1" * 64
        value = slot.sanitized_deployment(
            source_deployment("sub2api", "sub2api"),
            name="sub2api-blue",
            slot="blue",
            image=image,
            container_name="sub2api",
            annotations={"release": "test"},
        )
        selector = value["spec"]["selector"]["matchLabels"]
        labels = value["spec"]["template"]["metadata"]["labels"]
        self.assertEqual(selector[slot.SLOT_LABEL], "blue")
        self.assertEqual(labels[slot.SLOT_LABEL], "blue")
        self.assertEqual(value["spec"]["template"]["spec"]["containers"][0]["image"], image)

    def test_preview_service_does_not_copy_cluster_ip(self):
        service = {
            "metadata": {"name": "sub2api", "namespace": "sub2api-prod"},
            "spec": {
                "clusterIP": "10.0.0.1",
                "clusterIPs": ["10.0.0.1"],
                "selector": {"app.kubernetes.io/name": "sub2api", "pod-template-hash": "old"},
                "ports": [{"name": "http", "port": 8080, "targetPort": "http"}],
            },
        }
        value = slot.preview_service(service, name="preview", slot="green")
        self.assertNotIn("clusterIP", value["spec"])
        self.assertNotIn("pod-template-hash", value["spec"]["selector"])
        self.assertEqual(value["spec"]["selector"][slot.SLOT_LABEL], "green")

    def test_rendered_job_uses_only_immutable_images_and_three_minute_drain(self):
        image = "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "a" * 64
        args = argparse.Namespace(
            source_sha="b" * 40,
            version="0.2.19-cce-20261105c",
            pipeline_run="42",
            backend_image=image,
            insights_image=image,
            runner_image=image,
        )
        manifest, workload = render_release.render(args)
        job = workload["items"][1]
        container = job["spec"]["template"]["spec"]["containers"][0]
        self.assertEqual(container["image"], image)
        self.assertEqual(container["args"][-1], "180")
        self.assertEqual(job["spec"]["ttlSecondsAfterFinished"], 60)
        self.assertEqual(manifest["source_sha"], "b" * 40)
        embedded = json.loads(workload["items"][0]["data"]["cce-release-manifest.json"])
        self.assertEqual(embedded, manifest)


if __name__ == "__main__":
    unittest.main()
