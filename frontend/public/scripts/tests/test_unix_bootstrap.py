"""Exercise the piped installer with real Python and a local download fixture."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parents[1]
SCRIPT = SCRIPTS / "sub2api-local-setup.sh"
SPEC = importlib.util.spec_from_file_location("setup_helper", SCRIPTS / "sub2api-local-setup.py")
HELPER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HELPER)


@unittest.skipIf(os.name == "nt", "POSIX bootstrap")
class UnixBootstrapTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        for tool in ("mkdir", "mktemp", "rm", "mv", "uname", "tar", "chmod", "awk", "shasum"):
            executable = shutil.which(tool)
            if executable:
                (self.bin / tool).symlink_to(executable)
        (self.bin / "python3").symlink_to(sys.executable)
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("SUB2API_SETUP_")}
        self.env.update({
            "PATH": str(self.bin), "HOME": str(self.root),
            "TMPDIR": str(self.root), "XDG_CACHE_HOME": str(self.root / "cache"),
            "SUB2API_SETUP_CONFIG_DIR": str(self.root / "config"),
            "FIXTURE_SCRIPTS": str(SCRIPTS), "FIXTURE_LOG": str(self.root / "downloads.jsonl"),
        })
        curl = self.bin / "curl"
        curl.write_text("#!" + sys.executable + "\n" + '''
import json, os, pathlib, sys
args = sys.argv[1:]
with open(os.environ['FIXTURE_LOG'], 'a') as log:
    log.write(json.dumps(args) + '\\n')
url = next(arg for arg in args if arg.startswith('https://'))
if 'github.com' in url:
    sys.exit(28)
dest = pathlib.Path(args[args.index('-o') + 1])
source = pathlib.Path(os.environ['FIXTURE_SCRIPTS']) / url.rsplit('/', 1)[-1]
dest.write_bytes(b'corrupt' if os.environ.get('FIXTURE_CORRUPT') and source.suffix == '.whl' else source.read_bytes())
''')
        curl.chmod(0o755)

    def run_setup(self, client="codex"):
        result = subprocess.run(
            ["/bin/bash", "-s", "--", "https://api.example.test", "sk-test", client, "--yes", "--skip-doctor"],
            input=SCRIPT.read_text(), text=True, capture_output=True, env=self.env, timeout=15,
        )
        self.output = result.stdout + result.stderr
        self.downloads = [json.loads(line) for line in (self.root / "downloads.jsonl").read_text().splitlines()]
        self.assertEqual(list(self.root.glob("sub2api-setup.*")), [], self.output)
        return result

    def test_system_python_writes_and_preserves_config_without_runtime_download(self):
        config = self.root / "config/.codex/config.toml"
        config.parent.mkdir(parents=True)
        config.write_text('model = "old"\n[features]\ngoals = true\n[mcp_servers.example]\ncommand = "kept"\n')
        result = self.run_setup()
        self.assertEqual(result.returncode, 0, self.output)
        parsed = HELPER.toml_loads(config.read_text())
        self.assertEqual(parsed["mcp_servers"]["example"]["command"], "kept")
        self.assertEqual(parsed["model_providers"]["OpenAI"]["base_url"], "https://api.example.test/v1")
        self.assertEqual(len(list(config.parent.glob("*.sub2api-backup-*"))), 1)
        self.assertIn("使用系统 Python", self.output)
        self.assertNotIn("github.com", json.dumps(self.downloads))
        self.assertEqual(any(".whl" in arg for args in self.downloads for arg in args), sys.version_info < (3, 11))
        for args in self.downloads:
            self.assertIn("--connect-timeout", args)
            self.assertIn("--max-time", args)
            self.assertIn("--retry", args)

    def test_invalid_toml_is_rejected_before_existing_config_is_changed(self):
        config = self.root / "config/.codex/config.toml"
        config.parent.mkdir(parents=True)
        original = 'invalid = "unterminated\n'
        config.write_text(original)
        result = self.run_setup()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("不是有效 TOML", self.output)
        self.assertEqual(config.read_text(), original)
        self.assertEqual(list(config.parent.glob("*.sub2api-backup-*")), [])

    def test_json_client_needs_neither_toml_nor_portable_python(self):
        result = self.run_setup("claude")
        self.assertEqual(result.returncode, 0, self.output)
        self.assertEqual(len(self.downloads), 1)
        config = json.loads((self.root / "config/.claude/settings.json").read_text())
        self.assertEqual(config["env"]["ANTHROPIC_AUTH_TOKEN"], "sk-test")

    @unittest.skipUnless(sys.version_info < (3, 11), "bundled parser used by older Python")
    def test_corrupt_parser_is_rejected_without_writing_config(self):
        self.env["FIXTURE_CORRUPT"] = "1"
        result = self.run_setup()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SHA-256 校验失败", self.output)
        self.assertFalse((self.root / "config/.codex/config.toml").exists())

    def test_cached_symlink_runtime_is_used_when_system_python_is_unavailable(self):
        (self.bin / "python3").unlink()
        cache = self.root / "cache/sub2api/python/3.14.7+20260825/python/bin"
        cache.mkdir(parents=True)
        (cache / "python3").symlink_to(sys.executable)
        result = self.run_setup()
        self.assertEqual(result.returncode, 0, self.output)
        self.assertNotIn("github.com", json.dumps(self.downloads))

    def test_older_python3_is_skipped_in_favor_of_compatible_python(self):
        (self.bin / "python3").unlink()
        (self.bin / "python3").write_text("#!/bin/sh\nexit 1\n")
        (self.bin / "python3").chmod(0o755)
        (self.bin / "python").symlink_to(sys.executable)
        result = self.run_setup()
        self.assertEqual(result.returncode, 0, self.output)
        self.assertNotIn("github.com", json.dumps(self.downloads))
        self.assertIn("使用系统 Python: " + str(self.bin / "python") + "\n", self.output)

    def test_missing_python_reports_failed_bounded_download_and_leaves_no_config(self):
        (self.bin / "python3").unlink()
        result = self.run_setup()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("便携 Python 下载失败", self.output)
        self.assertIn("Python 3.8+", self.output)
        self.assertEqual(len(self.downloads), 1)
        self.assertIn("--connect-timeout", self.downloads[0])
        self.assertIn("--max-time", self.downloads[0])
        self.assertFalse((self.root / "config").exists())
