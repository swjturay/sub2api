import importlib.util
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path


SCRIPT = Path(__file__).parents[1] / "sub2api-local-setup.py"
WINDOWS_SCRIPT = Path(__file__).parents[1] / "sub2api-local-setup.ps1"
SPEC = importlib.util.spec_from_file_location("sub2api_local_setup", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
SPEC.loader.exec_module(MODULE)


class LocalSetupTests(unittest.TestCase):
    def test_minimax_opencode_uses_compatible_provider_for_native_and_composite_groups(self):
        for platform in ("minimax", "composite"):
            with self.subTest(platform=platform), tempfile.TemporaryDirectory() as root:
                models = {"MiniMax-M3": {"name": "MiniMax-M3"}, "minimax/custom": {"name": "Custom"}}
                result = json.loads(MODULE.opencode_update(
                    Path(root) / "opencode.json", "https://api.example.test/v1", "sk-test", platform, models,
                ))
                provider = result["provider"]["shared-ai-minimax"]
                self.assertEqual(provider["npm"], "@ai-sdk/openai-compatible")
                self.assertEqual(provider["options"]["baseURL"], "https://api.example.test/v1")
                self.assertEqual(set(provider["models"]), set(models))
                self.assertNotIn("shared-ai-openai", result["provider"])

    def test_minimax_discovery_failure_keeps_minimax_fallbacks(self):
        with patch.dict(os.environ, {"SUB2API_SETUP_OPENCODE_MODELS": ""}), patch.object(
            MODULE.urllib.request, "urlopen", side_effect=OSError("offline")
        ):
            models = MODULE.discover_opencode_models("https://api.example.test/v1", "sk-test", "minimax")
        self.assertEqual(next(iter(models)), "MiniMax-M3")
        self.assertNotIn("gpt-5.5", models)

    @unittest.skipUnless(shutil.which("powershell.exe"), "Windows PowerShell is required")
    def test_windows_codex_command_preserves_args_and_does_not_exit_host(self):
        with tempfile.TemporaryDirectory() as root:
            root_path = Path(root)
            fake_python = root_path / "python.cmd"
            fake_python.write_text(
                "@echo off\r\n"
                "if \"%~1\"==\"-c\" exit /b 0\r\n"
                "echo PY_HELPER_REACHED\r\n"
                "exit /b 23\r\n",
                encoding="ascii",
            )
            probe = root_path / "probe.ps1"
            probe.write_text(
                "function Invoke-WebRequest {\n"
                "  param([switch]$UseBasicParsing, $Uri, $OutFile)\n"
                "  [IO.File]::WriteAllText($OutFile, '# fixture')\n"
                "}\n"
                f"$source = [IO.File]::ReadAllText({json.dumps(str(WINDOWS_SCRIPT))})\n"
                "try {\n"
                "  & ([scriptblock]::Create($source)) 'https://api.example.test' 'sk-REDACTED' 'codex' --yes\n"
                "} catch {\n"
                "  Write-Output ('CAUGHT: ' + $_.Exception.Message)\n"
                "}\n"
                "Write-Output 'HOST_SURVIVED'\n",
                encoding="utf-8-sig",
            )
            env = os.environ.copy()
            env["PATH"] = root
            env["SUB2API_SETUP_CONFIG_DIR"] = root
            result = subprocess.run(
                ["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(probe)],
                capture_output=True,
                text=True,
                encoding="utf-8",
                errors="replace",
                check=False,
                env=env,
            )

        output = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0, output)
        self.assertIn("PY_HELPER_REACHED", output)
        self.assertIn("HOST_SURVIVED", output)
        self.assertNotIn("System.Char", output)

    @unittest.skipUnless(shutil.which("powershell.exe"), "Windows PowerShell is required")
    def test_windows_portable_python_branch_handles_an_empty_cache(self):
        source = WINDOWS_SCRIPT.read_text(encoding="utf-8")
        function_source = source.split("$client = $env:SUB2API_SETUP_CLIENT", 1)[0]
        with tempfile.TemporaryDirectory() as root:
            probe = Path(root) / "probe.ps1"
            probe.write_text(
                function_source
                + "\nfunction Invoke-WebRequest { param([switch]$UseBasicParsing, $Uri, $OutFile, $TimeoutSec) throw 'DOWNLOAD_REACHED' }\n"
                + "function Start-Sleep { param($Seconds) }\n"
                + "$env:SUB2API_SETUP_USE_CURL = 'false'\n"
                + f"$env:LOCALAPPDATA = {json.dumps(root)}\n"
                + "try { Get-PortablePython 'https://example.test' | Out-Null } catch { Write-Output $_.Exception.Message }\n",
                encoding="utf-8-sig",
            )
            result = subprocess.run(
                ["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(probe)],
                capture_output=True,
                encoding="utf-8", errors="replace",
                check=False,
            )

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("DOWNLOAD_REACHED", result.stdout)
        self.assertNotIn("LiteralPath", result.stdout + result.stderr)

    @unittest.skipUnless(shutil.which("powershell.exe"), "Windows PowerShell is required")
    def test_windows_download_retries_with_a_bounded_timeout_and_atomic_output(self):
        source = WINDOWS_SCRIPT.read_text(encoding="utf-8")
        function_source = source.split("function Find-Python", 1)[0]
        with tempfile.TemporaryDirectory() as root:
            output = Path(root) / "download.bin"
            probe = Path(root) / "probe.ps1"
            probe.write_text(
                function_source
                + "\n$script:attempts = 0\n"
                + "function Start-Sleep { param($Seconds) }\n"
                + "function Invoke-WebRequest { param([switch]$UseBasicParsing, $Uri, $OutFile, $TimeoutSec)\n"
                + "  $script:attempts += 1\n"
                + "  if ($TimeoutSec -ne 7) { throw 'BAD_TIMEOUT' }\n"
                + "  if ($script:attempts -eq 1) { throw 'TRANSIENT_FAILURE' }\n"
                + "  [IO.File]::WriteAllBytes($OutFile, [byte[]](1, 2, 3))\n"
                + "}\n"
                + "$env:SUB2API_SETUP_USE_CURL = 'false'\n"
                + f"Invoke-SetupDownload -Uri 'https://example.test/file' -OutFile {json.dumps(str(output))} -Label '测试文件' -TimeoutSec 7\n"
                + "Write-Output ('ATTEMPTS=' + $script:attempts)\n",
                encoding="utf-8-sig",
            )
            result = subprocess.run(
                ["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(probe)],
                capture_output=True,
                encoding="utf-8", errors="replace",
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("ATTEMPTS=2", result.stdout)
            self.assertEqual(output.read_bytes(), bytes((1, 2, 3)))

    def test_windows_downloads_have_stage_labels_and_curl_time_limits(self):
        source = WINDOWS_SCRIPT.read_text(encoding="utf-8")
        self.assertIn("正在下载${Label}", source)
        self.assertIn("--connect-timeout 15", source)
        self.assertIn("--max-time $TimeoutSec", source)
        self.assertIn("$attempt -le 3", source)
        self.assertIn("$ProgressPreference = 'SilentlyContinue'", source)
        self.assertIn("本站便携 Python（约 11 MiB，首次运行需要）", source)
        self.assertIn("https://www.python.org/ftp/python/$pythonVersion/$pythonAsset", source)

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
            path.write_text(
                "model_provider = \"legacy\"\nmodel_catalog_json = \"codex-models.json\"\n\n"
                "[model_providers.sub2api]\nname = \"Sub2API\"\n\n[features]\ngoals = true\n",
                encoding="utf-8",
            )
            result = MODULE.codex_update(path, "https://api.example.test/v1", "sk-new", "composite", "api-key", False)
        parsed = __import__("tomllib").loads(result)
        self.assertEqual(parsed["model_provider"], "OpenAI")
        self.assertNotIn("model_catalog_json", parsed)
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
        self.assertNotIn("sub2api", parsed["model_providers"])
        self.assertTrue(parsed["features"]["remote_compaction_v2"])
        self.assertTrue(parsed["features"]["image_generation"])
        self.assertTrue(parsed["features"]["goals"])

    def test_routed_codex_writes_the_api_key_directly(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "config.toml"
            result = MODULE.codex_update(
                path,
                "https://api.example.test/v1",
                "sk-routed",
                "composite",
                "api-key",
                False,
            )

        parsed = __import__("tomllib").loads(result)
        provider = parsed["model_providers"]["OpenAI"]
        self.assertEqual(provider["experimental_bearer_token"], "sk-routed")
        self.assertFalse(provider["requires_openai_auth"])
        self.assertEqual(provider["name"], "OpenAI")
        self.assertEqual(provider["http_headers"]["x-openai-actor-authorization"], "local-image-extension")
        self.assertNotIn("env_key", provider)

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
                self.assertNotIn("model_catalog_json", parsed)
                self.assertFalse(parsed["model_providers"]["OpenAI"]["requires_openai_auth"])
                self.assertEqual(parsed["model_providers"]["OpenAI"]["experimental_bearer_token"], "sk-new")
                self.assertFalse((Path(root) / ".codex/codex-models.json").exists())
            finally:
                os.environ.clear()
                os.environ.update(original)


if __name__ == "__main__":
    unittest.main()
