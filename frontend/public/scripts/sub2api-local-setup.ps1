$ErrorActionPreference = 'Stop'

$scriptVersion = '2026.09.07'
$pythonVersion = '3.13.15'
$pythonAsset = 'python-3.13.15-embed-amd64.zip'
$pythonSha256 = 'd1f04d990aee1253d8569e8e5104e30fa9f5fa830899f14843448872d936a2cf'

function Fail([string]$Message) {
  throw "[sub2api] 错误: $Message"
}

function Invoke-SetupDownload {
  param(
    [Parameter(Mandatory = $true)][string]$Uri,
    [Parameter(Mandatory = $true)][string]$OutFile,
    [Parameter(Mandatory = $true)][string]$Label,
    [int]$TimeoutSec = 600
  )

  $partial = "$OutFile.part-$([guid]::NewGuid().ToString('N'))"
  Write-Host "[sub2api] 正在下载${Label}（单次请求超时 ${TimeoutSec} 秒，失败自动重试）"
  try {
    $curl = if ($env:SUB2API_SETUP_USE_CURL -eq 'false') { $null } else { Get-Command curl.exe -ErrorAction SilentlyContinue }
    if ($null -ne $curl) {
      for ($attempt = 1; $attempt -le 3; $attempt++) {
        & $curl.Source --fail --location --show-error --progress-bar `
          --connect-timeout 15 --max-time $TimeoutSec --output $partial $Uri
        $curlExit = $LASTEXITCODE
        if ($curlExit -eq 0) { break }
        Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
        if ($attempt -eq 3 -or $curlExit -eq 22) { Fail "${Label}下载失败（curl exit $curlExit）" }
        Write-Host "[sub2api] ${Label}下载失败，第 $($attempt + 1)/3 次重试即将开始"
        Start-Sleep -Seconds (2 * $attempt)
      }
    } else {
      for ($attempt = 1; $attempt -le 3; $attempt++) {
        $previousProgressPreference = $ProgressPreference
        try {
          # Windows PowerShell 5.1 mislabels download progress as writing the request stream.
          $ProgressPreference = 'SilentlyContinue'
          Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $partial -TimeoutSec $TimeoutSec
          break
        } catch {
          if ($attempt -eq 3) { Fail "${Label}下载失败: $($_.Exception.Message)" }
          Write-Host "[sub2api] ${Label}下载失败，第 $($attempt + 1)/3 次重试即将开始"
          Start-Sleep -Seconds (2 * $attempt)
        } finally {
          $ProgressPreference = $previousProgressPreference
        }
      }
    }

    if (-not (Test-Path -LiteralPath $partial)) { Fail "${Label}下载后找不到临时文件" }
    $length = (Get-Item -LiteralPath $partial).Length
    if ($length -le 0) { Fail "${Label}下载结果为空" }
    Move-Item -LiteralPath $partial -Destination $OutFile -Force
    $size = if ($length -ge 1MB) { '{0:N1} MiB' -f ($length / 1MB) } else { '{0:N1} KiB' -f ($length / 1KB) }
    Write-Host "[sub2api] ${Label}下载完成（$size）"
  } finally {
    Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
  }
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

function Get-PortablePython([string]$Endpoint) {
  $cacheRoot = Join-Path $env:LOCALAPPDATA "Sub2API\python\$pythonVersion"
  $pythonPath = Get-ChildItem -LiteralPath $cacheRoot -Filter 'python.exe' -File -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
  if ([string]::IsNullOrWhiteSpace($pythonPath) -or -not (Test-Path -LiteralPath $pythonPath)) {
    New-Item -ItemType Directory -Force -Path $cacheRoot | Out-Null
    $archive = Join-Path $cacheRoot 'python.zip'
    $root = $Endpoint -replace '/v1/?$', ''
    $sources = @(
      @{ Uri = "$($root.TrimEnd('/'))/scripts/$pythonAsset"; Label = '本站便携 Python（约 11 MiB，首次运行需要）' },
      @{ Uri = "https://www.python.org/ftp/python/$pythonVersion/$pythonAsset"; Label = 'Python 官方便携运行时（约 11 MiB）' }
    )
    $downloaded = $false
    $lastDownloadError = $null
    foreach ($source in $sources) {
      try {
        Invoke-SetupDownload -Uri $source.Uri -OutFile $archive -Label $source.Label -TimeoutSec 300
        $downloaded = $true
        break
      } catch {
        $lastDownloadError = $_.Exception.Message
        Write-Host "[sub2api] $($source.Label)不可用，尝试下一个下载源"
      }
    }
    if (-not $downloaded) { Fail "便携 Python 获取失败。请检查网络，或先安装 Python 3.11+ 后重试。详情: $lastDownloadError" }
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant()
    if ($actual -ne $pythonSha256) { Remove-Item -LiteralPath $archive -Force; Fail '便携 Python SHA-256 校验失败' }
    try {
      Expand-Archive -LiteralPath $archive -DestinationPath $cacheRoot -Force
    } catch {
      Fail "便携 Python 解压失败: $($_.Exception.Message)"
    }
    $pythonPath = Get-ChildItem -LiteralPath $cacheRoot -Filter 'python.exe' -File -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
  }
  if ([string]::IsNullOrWhiteSpace($pythonPath) -or -not (Test-Path -LiteralPath $pythonPath)) { Fail '便携 Python 解压后找不到 python.exe' }
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

$endpoint = $env:SUB2API_SETUP_ENDPOINT
if ([string]::IsNullOrWhiteSpace($endpoint)) { Fail '缺少 SUB2API_SETUP_ENDPOINT' }
$python = Find-Python
if ($null -eq $python) { $python = Get-PortablePython $endpoint }
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
  Invoke-SetupDownload -Uri $helperUrl -OutFile $helper -Label '配置解析器' -TimeoutSec 60
  Write-Host '[sub2api] 配置解析器已下载，开始执行'
  & $python $helper @args
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
