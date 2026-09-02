$ErrorActionPreference = 'Stop'

$scriptVersion = '2026.09.01'
$pythonVersion = '3.14.7+20260825'
$pythonAsset = 'cpython-3.14.7+20260825-x86_64-pc-windows-msvc-install_only.tar.gz'
$pythonSha256 = 'a8a93fcf897f4c6ea4d120cf4a7ee4f98779d33a432a65b1af36a589c2f3d36c'

function Fail([string]$Message) {
  throw "[sub2api] 错误: $Message"
}

function Find-Python {
  foreach ($candidate in @('py', 'python', 'python3')) {
    $command = Get-Command $candidate -ErrorAction SilentlyContinue
    if ($null -eq $command) { continue }
    try {
      & $command.Source -c "import sys; raise SystemExit(0 if sys.version_info >= (3, 11) else 1)" 2>$null
      if ($LASTEXITCODE -eq 0) { return $command.Source }
    } catch { }
  }
  return $null
}

function Get-PortablePython {
  $cacheRoot = Join-Path $env:LOCALAPPDATA "Sub2API\python\$pythonVersion"
  $pythonPath = Get-ChildItem -LiteralPath $cacheRoot -Filter 'python.exe' -File -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
  if (-not (Test-Path -LiteralPath $pythonPath)) {
    New-Item -ItemType Directory -Force -Path $cacheRoot | Out-Null
    $archive = Join-Path $cacheRoot 'python.tar.gz'
    $url = "https://github.com/astral-sh/python-build-standalone/releases/download/20260825/$pythonAsset"
    Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $archive
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant()
    if ($actual -ne $pythonSha256) { Remove-Item -LiteralPath $archive -Force; Fail '便携 Python SHA-256 校验失败' }
    $tar = Get-Command tar.exe -ErrorAction SilentlyContinue
    if ($null -eq $tar) { Fail '系统缺少 tar.exe，无法解压便携 Python' }
    & $tar.Source -xzf $archive -C $cacheRoot
    if ($LASTEXITCODE -ne 0) { Fail '便携 Python 解压失败' }
    $pythonPath = Get-ChildItem -LiteralPath $cacheRoot -Filter 'python.exe' -File -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
  }
  if (-not (Test-Path -LiteralPath $pythonPath)) { Fail '便携 Python 解压后找不到 python.exe' }
  return $pythonPath
}

$client = $env:SUB2API_SETUP_CLIENT
$endpointArg = $null
$apiKeyArg = $null
if ([string]::IsNullOrWhiteSpace($env:SUB2API_SETUP_ENDPOINT) -and $args.Count -gt 0 -and -not $args[0].StartsWith('-')) {
  $endpointArg = $args[0]
  $args = $args | Select-Object -Skip 1
}
if ([string]::IsNullOrWhiteSpace($env:SUB2API_SETUP_API_KEY) -and $args.Count -gt 0 -and -not $args[0].StartsWith('-')) {
  $apiKeyArg = $args[0]
  $args = $args | Select-Object -Skip 1
}
if ($endpointArg) { $env:SUB2API_SETUP_ENDPOINT = $endpointArg }
if ($apiKeyArg) { $env:SUB2API_SETUP_API_KEY = $apiKeyArg }
if ([string]::IsNullOrWhiteSpace($client) -and $args.Count -gt 0 -and -not $args[0].StartsWith('-')) {
  $client = $args[0]
  $args = $args | Select-Object -Skip 1
}
$client = if ([string]::IsNullOrWhiteSpace($client)) { 'codex' } else { $client.ToLowerInvariant() }
switch ($client) {
  'codex-ws' { $env:SUB2API_SETUP_CLIENT = 'codex'; $env:SUB2API_SETUP_CODEX_WEBSOCKET = 'true' }
  'codex' { $env:SUB2API_SETUP_CLIENT = 'codex' }
  'claude' { $env:SUB2API_SETUP_CLIENT = 'claude' }
  'opencode' { $env:SUB2API_SETUP_CLIENT = 'opencode' }
  default { Fail "不支持的客户端: $client" }
}
if ($args.Count -gt 0 -and -not $args[0].StartsWith('-')) {
  $env:SUB2API_SETUP_PLATFORM = $args[0]
  $args = $args | Select-Object -Skip 1
}

$python = Find-Python
if ($null -eq $python) { $python = Get-PortablePython }
$endpoint = $env:SUB2API_SETUP_ENDPOINT
if ([string]::IsNullOrWhiteSpace($endpoint)) { Fail '缺少 SUB2API_SETUP_ENDPOINT' }
$helperUrl = $env:SUB2API_SETUP_PY_URL
if ([string]::IsNullOrWhiteSpace($helperUrl)) {
  $helperUrl = ($endpoint -replace '/v1/?$', '') + '/scripts/sub2api-local-setup.py'
}
Write-Host "[sub2api] 客户端: $($env:SUB2API_SETUP_CLIENT); Endpoint: $endpoint"
Write-Host "[sub2api] 正在准备配置解析器: $helperUrl"

$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("sub2api-setup-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
try {
  $helper = Join-Path $tempRoot 'sub2api-local-setup.py'
  Invoke-WebRequest -UseBasicParsing -Uri $helperUrl -OutFile $helper
  Write-Host '[sub2api] 配置解析器已下载，开始执行'
  & $python $helper @args
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
