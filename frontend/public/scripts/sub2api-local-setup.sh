#!/usr/bin/env sh
set -eu

SCRIPT_VERSION="2026.10.08"
TOMLI_WHEEL="tomli-2.2.1-py3-none-any.whl"
PYTHON_VERSION="3.14.7+20260825"
PYTHON_CACHE_ROOT="${XDG_CACHE_HOME:-${HOME:-.}/.cache}/sub2api/python/${PYTHON_VERSION}"

die() {
  printf '%s\n' "[sub2api] 错误: $*" >&2
  exit 1
}

command -v curl >/dev/null 2>&1 || die "需要 curl 才能下载配置解析器"

download() {
  printf '%s\n' "[sub2api] 正在下载 $3（单次超时 $4 秒，失败最多重试 2 次）" >&2
  if ! curl -fSL --proto '=https' --tlsv1.2 --connect-timeout 10 \
    --max-time "$4" --retry 2 --retry-delay 1 --retry-max-time "$4" \
    "$1" -o "$2.part"; then
    rm -f "$2.part"
    return 1
  fi
  [ -s "$2.part" ] || { rm -f "$2.part"; return 1; }
  mv "$2.part" "$2"
}

usable_python() {
  "$1" -c 'import sys; raise SystemExit(0 if sys.version_info >= (3, 8) else 1)' >/dev/null 2>&1
}

find_python() {
  for candidate in python3 python; do
    if command -v "$candidate" >/dev/null 2>&1 && usable_python "$candidate"; then
      printf '%s\n' "[sub2api] 使用系统 Python: $(command -v "$candidate")" >&2
      command -v "$candidate"
      return 0
    fi
  done
  return 1
}

runtime_spec() {
  os_name=$(uname -s)
  arch=$(uname -m)
  case "${os_name}:${arch}" in
    Linux:x86_64|Linux:amd64)
      PYTHON_ASSET="cpython-3.14.7+20260825-x86_64-unknown-linux-gnu-install_only.tar.gz"
      PYTHON_SHA256="d68dfa9c5d37afec0a4c8ffbf5c20d05d34492bd4561c94d7c3c7578e21a7f71"
      ;;
    Linux:aarch64|Linux:arm64)
      PYTHON_ASSET="cpython-3.14.7+20260825-aarch64-unknown-linux-gnu-install_only.tar.gz"
      PYTHON_SHA256="bd5e9541a62fd1143270a38c2c1cdb94c7a1015c6d104aefa9e12ab5f80370b2"
      ;;
    Darwin:x86_64)
      PYTHON_ASSET="cpython-3.14.7+20260825-x86_64-apple-darwin-install_only.tar.gz"
      PYTHON_SHA256="25fa61a196fee9b19e56a17ae68db40c11eabc57d0f3127bdded442651e14e80"
      ;;
    Darwin:arm64)
      PYTHON_ASSET="cpython-3.14.7+20260825-aarch64-apple-darwin-install_only.tar.gz"
      PYTHON_SHA256="4c4a4114bc35f9d76d194fd72f43d8375b2f30686ddfe6b40c9258cfe6c16e40"
      ;;
    *) die "暂不支持的系统架构: ${os_name} ${arch}" ;;
  esac
}

download_python() {
  runtime_spec
  mkdir -p "$PYTHON_CACHE_ROOT"
  archive="$temp_dir/python.tar.gz"
  # python3 is normally a symlink in python-build-standalone archives.
  python_path="$PYTHON_CACHE_ROOT/python/bin/python3"
  if ! usable_python "$python_path"; then
    printf '%s\n' "[sub2api] 未找到可用的 Python 3.8+，准备便携运行时" >&2
    url="https://github.com/astral-sh/python-build-standalone/releases/download/20260825/${PYTHON_ASSET}"
    download "$url" "$archive" "便携 Python（首次运行需要）" 180 || die "便携 Python 下载失败；请检查网络，或安装 Python 3.8+ 后重试"
    if command -v sha256sum >/dev/null 2>&1; then
      actual=$(sha256sum "$archive" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
      actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    else
      die "缺少 sha256sum 或 shasum，无法校验便携 Python"
    fi
    [ "$actual" = "$PYTHON_SHA256" ] || die "便携 Python SHA-256 校验失败"
    tar -xzf "$archive" -C "$PYTHON_CACHE_ROOT" || die "便携 Python 解压失败"
    chmod -R u+rwX,go-rwx "$PYTHON_CACHE_ROOT"
  fi
  usable_python "$python_path" || die "便携 Python 解压后不可执行或版本不兼容"
  printf '%s\n' "$python_path"
}

if [ -z "${SUB2API_SETUP_ENDPOINT:-}" ] && [ "${1:-}" != "" ] && [ "${1#-}" = "$1" ]; then
  export SUB2API_SETUP_ENDPOINT="$1"
  shift
fi
if [ -z "${SUB2API_SETUP_API_KEY:-}" ] && [ "${1:-}" != "" ] && [ "${1#-}" = "$1" ]; then
  export SUB2API_SETUP_API_KEY="$1"
  shift
fi

client="${SUB2API_SETUP_CLIENT:-}"
if [ -z "$client" ] && [ "${1:-}" != "" ] && [ "${1#-}" = "$1" ]; then
  client=$1
  shift
fi
client=${client:-codex}
case "$client" in
  codex-ws)
    export SUB2API_SETUP_CLIENT=codex
    export SUB2API_SETUP_CODEX_WEBSOCKET=true
    ;;
  codex|claude|opencode)
    export SUB2API_SETUP_CLIENT="$client"
    ;;
  *) die "不支持的客户端: $client" ;;
esac
if [ "${1:-}" != "" ] && [ "${1#-}" = "$1" ]; then
  export SUB2API_SETUP_PLATFORM="$1"
  shift
fi

endpoint=${SUB2API_SETUP_ENDPOINT:-}
[ -n "$endpoint" ] || die "缺少 SUB2API_SETUP_ENDPOINT"
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/sub2api-setup.XXXXXX")
trap 'rm -rf "$temp_dir"' EXIT HUP INT TERM
python_cmd=$(find_python || download_python)
helper_url=${SUB2API_SETUP_PY_URL:-${endpoint%/v1}/scripts/sub2api-local-setup.py}
printf '%s\n' "[sub2api] 客户端: ${SUB2API_SETUP_CLIENT}; Endpoint: ${endpoint}" >&2
printf '%s\n' "[sub2api] 正在准备配置解析器: ${helper_url}" >&2
helper="$temp_dir/sub2api-local-setup.py"
download "$helper_url" "$helper" "配置解析器" 60 || die "配置解析器下载失败"
if [ "$SUB2API_SETUP_CLIENT" = codex ] && ! "$python_cmd" -c 'import tomllib' >/dev/null 2>&1; then
  download "${helper_url%/*}/$TOMLI_WHEEL" "$temp_dir/$TOMLI_WHEEL" "TOML 解析库（约 14 KiB，无需安装）" 60 || die "TOML 解析库下载失败"
fi
printf '%s\n' "[sub2api] 配置解析器已下载，开始执行" >&2

"$python_cmd" "$helper" "$@"
