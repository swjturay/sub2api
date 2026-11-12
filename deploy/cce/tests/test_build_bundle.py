import importlib.util
import json
from pathlib import Path
import tarfile
import tempfile
import unittest


MODULE_PATH = Path(__file__).resolve().parents[1] / "build_bundle.py"
SPEC = importlib.util.spec_from_file_location("build_bundle", MODULE_PATH)
bundle = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(bundle)


class BuildBundleTest(unittest.TestCase):
    def test_build_is_reproducible_and_self_describing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            release_dir = root / "release"
            release_dir.mkdir()
            manifest = {
                "schema_version": "2",
                "source_ref": "cce-deploy",
                "source_sha": "a" * 40,
                "version": "0.2.21-cce-codearts-24",
                "created_at": "2026-11-12T12:00:00+00:00",
                "images": {
                    "backend": {
                        "immutable_image": "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "1" * 64,
                        "source_sha": "a" * 40,
                    },
                    "insights": {
                        "immutable_image": "registry.cn-shanghai.aliyuncs.com/rayless/sub2api@sha256:" + "2" * 64,
                        "source_sha": "a" * 40,
                    },
                },
            }
            (release_dir / "cce-release-manifest.json").write_text(
                json.dumps(manifest), encoding="utf-8"
            )
            release_tool = root / "release.py"
            release_tool.write_text("print('release')\n", encoding="utf-8")

            first = bundle.build(release_dir, release_tool, root / "first")
            second = bundle.build(release_dir, release_tool, root / "second")

            self.assertEqual(first["sha256"], second["sha256"])
            self.assertEqual(first["release_id"], "0.2.21-cce-codearts-24-aaaaaaaaaaaa")
            self.assertEqual(first["archive"].read_bytes(), second["archive"].read_bytes())
            self.assertEqual(first["checksum"].read_text(), first["sha256"] + "  " + first["archive"].name + "\n")

            with tarfile.open(first["archive"], "r:gz") as archive:
                self.assertEqual(
                    archive.getnames(),
                    ["bundle.json", "cce-release-manifest.json", "release.py"],
                )
                metadata = json.load(archive.extractfile("bundle.json"))
            self.assertEqual(metadata["schema_version"], 1)
            self.assertEqual(metadata["source_sha"], "a" * 40)
            self.assertEqual(set(metadata["files"]), {"cce-release-manifest.json", "release.py"})


if __name__ == "__main__":
    unittest.main()
