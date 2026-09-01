#!/usr/bin/env python3
"""Apply Sub2API client configuration without third-party Python packages."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request
from urllib.parse import urlencode, urlsplit, urlunsplit
from pathlib import Path
from typing import Any, NoReturn


SCRIPT_VERSION = "2026.08.31"
BACKUP_RE = re.compile(r"^(?P<original>.+)\.sub2api-backup-(?P<stamp>\d{8}-\d{6})$")
TOML_SECTION_RE = re.compile(r"^\s*\[([^\]]+)\]\s*(?:#.*)?$")
TOML_KEY_RE = re.compile(r"^\s*([A-Za-z0-9_-]+)\s*=")

if os.name == "nt":
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace")
        except (AttributeError, OSError):
            pass


class SetupError(RuntimeError):
    pass


def log(message: str) -> None:
    print(f"[sub2api] {message}")


def fail(message: str) -> NoReturn:
    raise SetupError(message)


def expand_path(value: str) -> Path:
    expanded = os.path.expandvars(os.path.expanduser(value.strip()))
    return Path(expanded).resolve()


def config_root() -> Path:
    override = os.environ.get("SUB2API_SETUP_CONFIG_DIR", "").strip()
    return expand_path(override) if override else Path.home()


def env_required(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        fail(f"缺少环境变量 {name}")
    return value


def safe_endpoint(endpoint: str) -> str:
    parsed = urlsplit(endpoint)
    if not parsed.scheme or not parsed.netloc:
        return endpoint
    return urlunsplit((parsed.scheme, parsed.netloc, parsed.path, "", ""))


def discover_opencode_models(endpoint: str, api_key: str, platform: str) -> dict[str, Any]:
    selected = [item.strip() for item in os.environ.get("SUB2API_SETUP_OPENCODE_MODELS", "").split(",") if item.strip()]
    ids = selected
    if not ids:
        request = urllib.request.Request(
            endpoint.rstrip("/") + "/models",
            headers={"Accept": "application/json", "Authorization": f"Bearer {api_key}"},
        )
        try:
            with urllib.request.urlopen(request, timeout=20) as response:
                payload = json.load(response)
            values = payload.get("data", payload.get("models", [])) if isinstance(payload, dict) else []
            ids = [
                str(item.get("id") or item.get("name", "")).replace("models/", "", 1)
                for item in values
                if isinstance(item, dict) and (item.get("id") or item.get("name"))
            ]
        except (OSError, ValueError, urllib.error.URLError):
            return {}

    priority = {
        "openai": ["gpt-5.5", "gpt-5.6", "gpt-5.4-mini"],
        "anthropic": ["claude-sonnet-4-6", "claude-opus-4-6-thinking", "claude-fable-5"],
        "gemini": ["gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"],
        "antigravity": ["claude-sonnet-4-6", "gemini-3.1-pro-high", "gemini-2.5-flash"],
        "grok": ["grok-4.5", "grok-build-0.1", "grok-4.20-multi-agent-0309"],
        "deepseek": ["deepseek-v4-pro", "deepseek-chat"],
        "composite": ["gpt-5.5", "claude-sonnet-4-6", "gemini-2.5-pro"],
    }
    unique = list(dict.fromkeys(ids))
    ordered = [item for item in priority.get(platform, []) if item in unique]
    ordered.extend(item for item in unique if item not in ordered)
    return {item: {"name": item} for item in ordered[:3]}


def fetch_codex_model_catalog(endpoint: str, api_key: str) -> str | None:
    request_url = endpoint.rstrip("/") + "/models?" + urlencode({"client_version": "0.152.0"})
    request = urllib.request.Request(
        request_url,
        headers={"Accept": "application/json", "Authorization": f"Bearer {api_key}"},
    )
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            payload = json.load(response)
    except (OSError, ValueError, urllib.error.URLError) as exc:
        log(f"警告: Codex 模型目录获取失败，将保留已有目录: {exc}")
        return None
    if not isinstance(payload, dict) or not isinstance(payload.get("models"), list):
        log("警告: Codex 模型目录响应格式无效，将保留已有目录")
        return None
    return json.dumps(payload, ensure_ascii=False, indent=2) + "\n"


def provider_name(platform: str) -> str:
    return {
        "anthropic": "anthropic",
        "openai": "openai",
        "gemini": "gemini",
        "antigravity": "antigravity-claude",
        "grok": "grok",
        "deepseek": "openai",
        "composite": "openai",
    }.get(platform, "openai")


def normalize_endpoint(endpoint: str, client: str, platform: str) -> str:
    """Turn the site root into the client-specific API base URL."""
    root = endpoint.strip().rstrip("/")
    root = re.sub(r"/v1beta$|/v1$", "", root, flags=re.IGNORECASE)
    if platform == "antigravity":
        root += "/antigravity"
    if client == "claude":
        # Claude Code appends /v1/messages itself.
        return root
    if client == "opencode" and platform == "gemini":
        return root + "/v1beta"
    return root + "/v1"


def client_command(client: str) -> str:
    return {"claude": "claude", "codex": "codex", "opencode": "opencode"}[client]


def is_client_available(client: str) -> bool:
    return shutil.which(client_command(client)) is not None


def json_text(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, separators=(", ", ": "))


def toml_string(value: str) -> str:
    return json.dumps(value, ensure_ascii=False)


def find_section(lines: list[str], section: str | None) -> tuple[int, int] | None:
    if section is None:
        start = 0
        end = len(lines)
        for index, line in enumerate(lines):
            if TOML_SECTION_RE.match(line):
                end = index
                break
        return start, end

    start: int | None = None
    for index, line in enumerate(lines):
        match = TOML_SECTION_RE.match(line)
        if match and match.group(1).strip() == section:
            if start is not None:
                fail(f"TOML 配置段 [{section}] 出现多次，已中止")
            start = index
    if start is None:
        return None

    end = len(lines)
    for index in range(start + 1, len(lines)):
        if TOML_SECTION_RE.match(lines[index]):
            end = index
            break
    return start, end


def upsert_toml_values(
    text: str,
    section: str | None,
    values: dict[str, str],
    remove: set[str] | None = None,
) -> str:
    """Update only unique keys in a TOML section while retaining other text."""
    lines = text.splitlines()
    had_final_newline = text.endswith("\n") or not text
    bounds = find_section(lines, section)
    if bounds is None:
        if lines and lines[-1].strip():
            lines.append("")
        if section is not None:
            lines.append(f"[{section}]")
        lines.extend(f"{key} = {value}" for key, value in values.items())
    else:
        start, end = bounds
        found: dict[str, int] = {}
        for index in range(start if section is None else start + 1, end):
            match = TOML_KEY_RE.match(lines[index])
            if not match:
                continue
            key = match.group(1)
            if key in values or (remove and key in remove):
                if key in found:
                    fail(f"TOML 配置项 {key} 出现多次，已中止")
                found[key] = index

        if remove:
            lines = [
                line for index, line in enumerate(lines)
                if not (start <= index < end and (TOML_KEY_RE.match(line) or False) and TOML_KEY_RE.match(line).group(1) in remove)
            ]
            bounds = find_section(lines, section)
            if bounds is None:
                fail(f"TOML 配置段 [{section or 'root'}] 在删除字段后不可定位")
            start, end = bounds
            found = {}
            for index in range(start if section is None else start + 1, end):
                match = TOML_KEY_RE.match(lines[index])
                if match and match.group(1) in values:
                    if match.group(1) in found:
                        fail(f"TOML 配置项 {match.group(1)} 出现多次，已中止")
                    found[match.group(1)] = index

        missing: list[str] = []
        for key, value in values.items():
            if key in found:
                lines[found[key]] = f"{key} = {value}"
            else:
                missing.append(f"{key} = {value}")
        if missing:
            lines[end:end] = missing

    result = "\n".join(lines)
    return result + ("\n" if had_final_newline else "")


def validate_json(text: str, path: Path) -> None:
    try:
        json.loads(text)
    except json.JSONDecodeError as exc:
        fail(f"{path} 写入后不是有效 JSON: {exc}")


def validate_toml(text: str, path: Path) -> None:
    try:
        import tomllib

        tomllib.loads(text)
    except Exception as exc:  # tomllib raises TOMLDecodeError, Python 3.11+ only.
        fail(f"{path} 写入后不是有效 TOML: {exc}")


def load_json_object(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        fail(f"{path} 不是有效 JSON: {exc}")
    if not isinstance(value, dict):
        fail(f"{path} 根节点必须是 JSON 对象")
    return value


def claude_update(path: Path, endpoint: str, api_key: str) -> str:
    value = load_json_object(path)
    env = value.setdefault("env", {})
    if not isinstance(env, dict):
        fail(f"{path} 的 env 必须是 JSON 对象")
    env.update({
        "ANTHROPIC_BASE_URL": endpoint,
        "ANTHROPIC_AUTH_TOKEN": api_key,
        "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    })
    return json.dumps(value, ensure_ascii=False, indent=2) + "\n"


def opencode_update(path: Path, endpoint: str, api_key: str, platform: str, models: dict[str, Any]) -> str:
    value = load_json_object(path)
    providers = value.setdefault("provider", {})
    if not isinstance(providers, dict):
        fail(f"{path} 的 provider 必须是 JSON 对象")
    name = provider_name(platform)
    provider: dict[str, Any] = {
        "options": {"baseURL": endpoint, "apiKey": api_key},
    }
    if models:
        provider["models"] = models
    providers[name] = provider
    value.setdefault("$schema", "https://opencode.ai/config.json")
    return json.dumps(value, ensure_ascii=False, indent=2) + "\n"


def codex_update(
    path: Path,
    endpoint: str,
    api_key: str,
    platform: str,
    mode: str,
    websocket: bool,
    catalog_path: Path | None = None,
) -> str:
    text = path.read_text(encoding="utf-8") if path.exists() else ""
    routed = platform != "openai"
    section = "model_providers.sub2api" if routed else "model_providers.OpenAI"
    provider_label = "Sub2API" if routed else "OpenAI"
    values = {
        "name": toml_string(provider_label),
        "base_url": toml_string(endpoint),
        "wire_api": toml_string("responses"),
        "supports_websockets": "true" if websocket else "false",
        "supports_standalone_web_search": "true",
    }
    remove: set[str] = set()
    if routed or mode == "env":
        values.update({
            "requires_openai_auth": "false",
            "env_key": toml_string("SUB2API_API_KEY"),
        })
        remove.update({"experimental_bearer_token", "http_headers"})
    elif mode == "api-key":
        values.update({
            "requires_openai_auth": "false",
            "experimental_bearer_token": toml_string(api_key),
            "http_headers": '{ "x-openai-actor-authorization" = "local-image-extension" }',
        })
        remove.add("env_key")
    else:
        values["requires_openai_auth"] = "true"
        remove.update({"env_key", "experimental_bearer_token", "http_headers"})

    text = upsert_toml_values(
        text,
        None,
        {
            "model_provider": toml_string("sub2api" if routed else "OpenAI"),
            "model": toml_string("gpt-5.6-terra"),
            "web_search": toml_string("live"),
            "disable_response_storage": "true",
            "network_access": toml_string("enabled"),
        },
        {
            "review_model",
            "windows_wsl_setup_acknowledged",
            "model_context_window",
            "model_auto_compact_token_limit",
        },
    )
    if catalog_path is not None:
        text = upsert_toml_values(text, None, {
            "model_catalog_json": toml_string(str(catalog_path)),
        })
    text = upsert_toml_values(text, section, values, remove)
    return upsert_toml_values(text, "features", {
        "remote_compaction_v2": "true",
        "image_generation": "true",
    })


def make_targets(client: str, endpoint: str, api_key: str, platform: str, models: dict[str, Any]) -> dict[Path, str]:
    root = config_root()
    if client == "claude":
        return {root / ".claude" / "settings.json": claude_update(root / ".claude" / "settings.json", endpoint, api_key)}
    if client == "opencode":
        return {root / ".config" / "opencode" / "opencode.json": opencode_update(root / ".config" / "opencode" / "opencode.json", endpoint, api_key, platform, models)}
    mode = os.environ.get("SUB2API_SETUP_CODEX_AUTH_MODE", "api-key").strip() or "api-key"
    websocket = os.environ.get("SUB2API_SETUP_CODEX_WEBSOCKET", "false").lower() == "true"
    codex_path = root / ".codex" / "config.toml"
    catalog_path = root / ".codex" / "codex-models.json"
    catalog = fetch_codex_model_catalog(endpoint, api_key)
    catalog_available = catalog is not None or catalog_path.exists()
    if catalog_available:
        log(f"Codex 模型目录: {catalog_path}")
    targets: dict[Path, str] = {}
    if catalog is not None:
        targets[catalog_path] = catalog
    targets[codex_path] = codex_update(
        codex_path,
        endpoint,
        api_key,
        platform,
        mode,
        websocket,
        catalog_path if catalog_available else None,
    )
    if mode == "legacy":
        targets[root / ".codex" / "auth.json"] = json.dumps({"OPENAI_API_KEY": api_key}, indent=2) + "\n"
    return targets


def backup_path(path: Path, stamp: str) -> Path:
    return path.with_name(f"{path.name}.sub2api-backup-{stamp}")


def write_atomic(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    mode = stat.S_IRUSR | stat.S_IWUSR
    with tempfile.NamedTemporaryFile("w", encoding="utf-8", dir=path.parent, delete=False) as handle:
        temp_path = Path(handle.name)
        handle.write(text)
    try:
        os.chmod(temp_path, mode)
        os.replace(temp_path, path)
        if os.name != "nt":
            os.chmod(path, mode)
    finally:
        temp_path.unlink(missing_ok=True)


def validate_target(path: Path, text: str) -> None:
    if path.suffix.lower() == ".json":
        validate_json(text, path)
    elif path.suffix.lower() == ".toml":
        validate_toml(text, path)
    else:
        fail(f"不支持的配置文件类型: {path}")


def ask_to_continue(existing: list[Path], yes: bool) -> None:
    if not existing or yes:
        return
    log("以下配置文件已存在，将先备份后更新:")
    for path in existing:
        print(f"  - {path}")
    answer = input("继续吗？[Y/n] ").strip().lower()
    if answer not in {"", "y", "yes"}:
        fail("用户取消了配置修改")


def run_codex_doctor(root: Path) -> bool:
    executable = shutil.which("codex")
    if not executable:
        log("警告: 未找到 codex，跳过 codex doctor。")
        return True

    doctor_env = os.environ.copy()
    doctor_env["CODEX_HOME"] = str(root / ".codex")
    try:
        result = subprocess.run(
            [executable, "doctor", "--json"],
            env=doctor_env,
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        log(f"警告: codex doctor 无法执行，保留已写入配置: {exc}")
        return True

    try:
        report = json.loads(result.stdout)
    except json.JSONDecodeError:
        log(f"警告: codex doctor 输出不可解析，保留已写入配置 (exit={result.returncode})")
        return True

    checks = report.get("checks", {}) if isinstance(report, dict) else {}
    config_status = checks.get("config.load", {}).get("status", "unknown")
    auth_status = checks.get("auth.credentials", {}).get("status", "unknown")
    overall = report.get("overallStatus", "unknown") if isinstance(report, dict) else "unknown"
    log(f"codex doctor: overall={overall}, config.load={config_status}, auth.credentials={auth_status}")
    if config_status not in {"ok", "pass", "healthy"}:
        error = checks.get("config.load", {}).get("summary", "配置未被 Codex 接受")
        log(f"错误: codex doctor 配置检查失败: {error}")
        return False
    if auth_status not in {"ok", "pass", "healthy"}:
        error = checks.get("auth.credentials", {}).get("summary", "认证配置未被 Codex 接受")
        log(f"错误: codex doctor 认证检查失败: {error}")
        return False
    if overall == "warning":
        log("提示: codex doctor 存在非阻塞 warning，但配置和认证检查通过。")
    elif overall not in {"ok", "pass", "healthy"}:
        failed_checks = [
            f"{name}={check.get('status', 'unknown')}"
            for name, check in checks.items()
            if isinstance(check, dict)
            and check.get("status") not in {"ok", "pass", "healthy"}
            and name not in {"config.load", "auth.credentials"}
        ]
        suffix = ", ".join(failed_checks[:5]) or "optional/runtime check"
        log(f"警告: codex doctor overall={overall} ({suffix})，但配置和认证检查通过，保留已写入配置。")
    return True


def apply_setup(args: argparse.Namespace) -> int:
    client = env_required("SUB2API_SETUP_CLIENT").lower()
    if client not in {"claude", "codex", "opencode"}:
        fail(f"不支持的客户端: {client}")
    platform = os.environ.get("SUB2API_SETUP_PLATFORM", "").strip().lower()
    if not platform:
        platform = "openai" if client == "codex" else "anthropic"
    endpoint = normalize_endpoint(env_required("SUB2API_SETUP_ENDPOINT"), client, platform)
    api_key = env_required("SUB2API_SETUP_API_KEY")
    root = config_root()
    log(f"脚本版本: {SCRIPT_VERSION}")
    log(f"客户端: {client}; 平台: {platform}")
    log(f"Endpoint: {safe_endpoint(endpoint)}")
    log(f"配置根目录: {root}")
    models = discover_opencode_models(endpoint, api_key, platform) if client == "opencode" else {}
    if client == "opencode":
        log(f"OpenCode 推荐模型: {len(models)} 个")

    if not is_client_available(client):
        log(f"警告: 未检测到 {client_command(client)}，仍继续写入配置。")

    targets = make_targets(client, endpoint, api_key, platform, models)
    log(f"目标文件: {len(targets)} 个")
    existing = [path for path in targets if path.exists()]
    if existing:
        log(f"检测到已有文件: {len(existing)} 个，将先备份。")
    else:
        log("未检测到已有目标文件，将创建新配置。")
    ask_to_continue(existing, args.yes)

    for path, text in targets.items():
        log(f"校验候选配置: {path}")
        validate_target(path, text)

    if args.dry_run:
        for path, text in targets.items():
            action = "更新" if path.exists() else "创建"
            log(f"预演: {action} {path} ({len(text.encode('utf-8'))} bytes)")
        return 0

    stamp = dt.datetime.now().strftime("%Y%m%d-%H%M%S")
    log(f"备份批次: {stamp}")
    backups: dict[Path, Path] = {}
    created: list[Path] = []
    try:
        for path in targets:
            if path.exists():
                destination = backup_path(path, stamp)
                shutil.copy2(path, destination)
                backups[path] = destination
                log(f"已备份 {path} -> {destination}")
        for path, text in targets.items():
            if not path.exists():
                created.append(path)
            write_atomic(path, text)
            validate_target(path, path.read_text(encoding="utf-8"))
            log(f"已写入 {path}")
        if client == "codex" and not getattr(args, "skip_doctor", False):
            if not run_codex_doctor(root):
                raise SetupError("codex doctor 未通过，配置已回滚")
    except Exception as exc:
        log(f"写入失败，正在恢复: {exc}")
        for path in created:
            path.unlink(missing_ok=True)
        for path, backup in backups.items():
            try:
                shutil.copy2(backup, path)
            except OSError as restore_exc:
                log(f"恢复失败 {path}: {restore_exc}")
        raise SetupError(str(exc)) from exc

    log(f"配置完成，脚本版本 {SCRIPT_VERSION}")
    return 0


def list_backups() -> int:
    root = config_root()
    found = sorted(root.rglob("*.sub2api-backup-*"))
    if not found:
        log(f"{root} 下没有找到备份")
        return 0
    for path in found:
        print(path)
    return 0


def restore_backup(value: str) -> int:
    backup = expand_path(value)
    match = BACKUP_RE.match(str(backup))
    if not match or not backup.is_file():
        fail("请提供有效的 .sub2api-backup-YYYYMMDD-HHMMSS 文件路径")
    original = Path(match.group("original"))
    original.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(backup, original)
    log(f"已恢复 {backup} -> {original}")
    return 0


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description="Configure Sub2API clients locally")
    result.add_argument("--yes", action="store_true", help="skip confirmation for existing files")
    result.add_argument("--dry-run", action="store_true", help="validate and show changes without writing")
    result.add_argument("--list-backups", action="store_true")
    result.add_argument("--restore", metavar="BACKUP_FILE")
    result.add_argument("--skip-doctor", action="store_true", help="skip automatic codex doctor verification")
    return result


def main() -> int:
    args = parser().parse_args()
    try:
        if args.list_backups:
            return list_backups()
        if args.restore:
            return restore_backup(args.restore)
        return apply_setup(args)
    except SetupError as exc:
        print(f"[sub2api] 错误: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
