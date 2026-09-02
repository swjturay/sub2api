import importlib.util
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).parents[1] / "sub2api-local-setup.py"
WINDOWS_SCRIPT = Path(__file__).parents[1] / "sub2api-local-setup.ps1"
SPEC = importlib.util.spec_from_file_location("sub2api_local_setup", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
SPEC.loader.exec_module(MODULE)


class LocalSetupTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("powershell.exe"), "Windows PowerShell is required")
    def test_windows_portable_python_branch_handles_an_empty_cache(self):
        source = WINDOWS_SCRIPT.read_text(encoding="utf-8")
        function_source = source.split("$client = $env:SUB2API_SETUP_CLIENT", 1)[0]
        with tempfile.TemporaryDirectory() as root:
            probe = Path(root) / "probe.ps1"
            probe.write_text(
                function_source
                + "\nfunction Invoke-WebRequest { param([switch]$UseBasicParsing, $Uri, $OutFile) throw 'DOWNLOAD_REACHED' }\n"
                + f"$env:LOCALAPPDATA = {json.dumps(root)}\n"
                + "try { Get-PortablePython | Out-Null } catch { Write-Output $_.Exception.Message }\n",
                encoding="utf-8-sig",
            )
            result = subprocess.run(
                ["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(probe)],
                capture_output=True,
                text=True,
                check=False,
            )

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("DOWNLOAD_REACHED", result.stdout)
        self.assertNotIn("LiteralPath", result.stdout + result.stderr)

    def test_claude_update_preserves_unmanaged_settings(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "settings.json"
            path.write_text(json.dumps({"permissions": {"allow": ["Bash"]}, "env": {"CUSTOM": "keep"}}), encoding="utf-8")
            result = json.loads(MODULE.claude_update(path, "https://api.example.test", "sk-new"))
            self.assertEqual(result["permissions"]["allow"], ["Bash"])
            self.assertEqual(result["env"]["CUSTOM"], "keep")
            self.assertEqual(result["env"]["ANTHROPIC_AUTH_TOKEN"], "sk-new")

    def test_opencode_update_uses_shared_provider_for_openai_models(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "opencode.json"
            path.write_text(json.dumps({"provider": {"other": {"options": {"apiKey": "keep"}}}}), encoding="utf-8")
            result = json.loads(MODULE.opencode_update(
                path,
                "https://api.example.test/v1",
                "sk-new",
                "openai",
                {"gpt-test": {"name": "Test"}},
            ))
            self.assertEqual(result["provider"]["other"]["options"]["apiKey"], "keep")
            self.assertEqual(result["provider"]["shared-ai-openai"]["options"]["apiKey"], "sk-new")
            self.assertEqual(result["provider"]["shared-ai-openai"]["npm"], "@ai-sdk/openai")
            self.assertIn("gpt-test", result["provider"]["shared-ai-openai"]["models"])
            self.assertNotIn("openai", result["provider"])

    def test_opencode_update_groups_composite_models_by_provider(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "opencode.json"
            result = json.loads(MODULE.opencode_update(
                path,
                "https://api.example.test/v1",
                "sk-new",
                "composite",
                {
                    "gpt-5.5": {"name": "gpt-5.5"},
                    "claude-sonnet-4-6": {"name": "claude-sonnet-4-6"},
                    "gemini-2.5-pro": {"name": "gemini-2.5-pro"},
                },
            ))

        providers = result["provider"]
        self.assertEqual(set(providers), {"shared-ai-openai", "shared-ai-anthropic", "shared-ai-gemini"})
        self.assertEqual(providers["shared-ai-openai"]["npm"], "@ai-sdk/openai")
        self.assertEqual(providers["shared-ai-anthropic"]["npm"], "@ai-sdk/anthropic")
        self.assertEqual(providers["shared-ai-gemini"]["npm"], "@ai-sdk/google")
        self.assertEqual(providers["shared-ai-gemini"]["options"]["baseURL"], "https://api.example.test/v1beta")

    def test_codex_update_uses_nested_provider_section_and_valid_toml(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "config.toml"
            path.write_text("model_provider = \"legacy\"\n\n[features]\ngoals = true\n", encoding="utf-8")
            result = MODULE.codex_update(path, "https://api.example.test/v1", "sk-new", "openai", "api-key", False)
        parsed = __import__("tomllib").loads(result)
        self.assertEqual(parsed["model_provider"], "OpenAI")
        self.assertEqual(parsed["model"], "gpt-5.6-terra")
        self.assertNotIn("review_model", parsed)
        self.assertTrue(parsed["disable_response_storage"])
        self.assertEqual(parsed["network_access"], "enabled")
        self.assertNotIn("windows_wsl_setup_acknowledged", parsed)
        self.assertNotIn("model_context_window", parsed)
        self.assertNotIn("model_auto_compact_token_limit", parsed)
        self.assertEqual(parsed["model_providers"]["OpenAI"]["base_url"], "https://api.example.test/v1")
        self.assertEqual(parsed["model_providers"]["OpenAI"]["experimental_bearer_token"], "sk-new")
        self.assertTrue(parsed["model_providers"]["OpenAI"]["supports_standalone_web_search"])
        self.assertTrue(parsed["features"]["remote_compaction_v2"])
        self.assertTrue(parsed["features"]["image_generation"])
        self.assertTrue(parsed["features"]["goals"])

    def test_upsert_rejects_duplicate_managed_toml_keys(self):
        with self.assertRaises(MODULE.SetupError):
            MODULE.upsert_toml_values(
                "[model_providers.OpenAI]\nbase_url = \"one\"\nbase_url = \"two\"\n",
                "model_providers.OpenAI",
                {"base_url": '"new"'},
            )

    def test_codex_defaults_to_openai_api_key_mode_without_optional_environment(self):
        with tempfile.TemporaryDirectory() as root:
            original = dict(os.environ)
            try:
                for key in ("SUB2API_SETUP_PLATFORM", "SUB2API_SETUP_CODEX_AUTH_MODE", "SUB2API_SETUP_CODEX_WEBSOCKET"):
                    os.environ.pop(key, None)
                os.environ.update({
                    "SUB2API_SETUP_CONFIG_DIR": root,
                    "SUB2API_SETUP_CLIENT": "codex",
                    "SUB2API_SETUP_ENDPOINT": "https://api.example.test",
                    "SUB2API_SETUP_API_KEY": "sk-new",
                })
                MODULE.apply_setup(type("Args", (), {"yes": True, "dry_run": False, "skip_doctor": True})())
                parsed = __import__("tomllib").loads((Path(root) / ".codex/config.toml").read_text())
                self.assertEqual(parsed["model_provider"], "OpenAI")
                self.assertFalse(parsed["model_providers"]["OpenAI"]["requires_openai_auth"])
                self.assertEqual(parsed["model_providers"]["OpenAI"]["experimental_bearer_token"], "sk-new")
            finally:
                os.environ.clear()
                os.environ.update(original)


if __name__ == "__main__":
    unittest.main()
