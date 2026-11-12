import hashlib
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch


MODULE = Path(__file__).resolve().parents[1] / "stage_setup_runtime.py"
SPEC = importlib.util.spec_from_file_location("stage_setup_runtime", MODULE)
runtime = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(runtime)


class SetupRuntimeTests(unittest.TestCase):
    def fixture(self, root, payload=b"verified runtime"):
        scripts = root / "frontend/public/scripts"
        scripts.mkdir(parents=True)
        checksum = hashlib.sha256(payload).hexdigest()
        asset = "python-3.13.15-embed-amd64.zip"
        (scripts / "sub2api-local-setup.ps1").write_text(
            f"$pythonVersion = '3.13.15'\n$pythonAsset = '{asset}'\n$pythonSha256 = '{checksum}'\n"
        )
        return scripts / asset, checksum

    def test_verified_download_is_staged_at_the_served_path(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            destination, checksum = self.fixture(root)
            opener = Mock(return_value=io.BytesIO(b"verified runtime"))
            result = runtime.stage(root, opener)
            self.assertEqual(destination.read_bytes(), b"verified runtime")
            self.assertEqual(result["sha256"], checksum)
            opener.assert_called_once_with(
                "https://www.python.org/ftp/python/3.13.15/python-3.13.15-embed-amd64.zip", timeout=60
            )

    def test_valid_cached_runtime_does_not_download(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            destination, _ = self.fixture(root)
            destination.write_bytes(b"verified runtime")
            opener = Mock(side_effect=AssertionError("unnecessary download"))
            self.assertTrue(runtime.stage(root, opener)["cached"])
            opener.assert_not_called()

    def test_corrupt_download_cannot_replace_an_existing_asset(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            destination, _ = self.fixture(root)
            destination.write_bytes(b"previous asset")
            with self.assertRaisesRegex(ValueError, "SHA-256 mismatch"):
                runtime.stage(root, Mock(return_value=io.BytesIO(b"corrupt")))
            self.assertEqual(destination.read_bytes(), b"previous asset")
            self.assertEqual(list(destination.parent.glob("*.part")), [])

    def test_asset_mismatch_fails_before_network_access(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            destination, _ = self.fixture(root)
            bootstrap = destination.parent / "sub2api-local-setup.ps1"
            bootstrap.write_text(bootstrap.read_text().replace("embed-amd64", "../unexpected"))
            opener = Mock()
            with self.assertRaisesRegex(ValueError, "asset or checksum"):
                runtime.stage(root, opener)
            opener.assert_not_called()

    def test_oversized_download_is_rejected_and_cleaned_up(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            destination, _ = self.fixture(root)
            with patch.object(runtime, "MAX_BYTES", 3):
                with self.assertRaisesRegex(ValueError, "size limit"):
                    runtime.stage(root, Mock(return_value=io.BytesIO(b"four")))
            self.assertFalse(destination.exists())
            self.assertEqual(list(destination.parent.glob("*.part")), [])
