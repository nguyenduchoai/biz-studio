# Optional SDK runtime; interpreter selected by the app, not Windows Store aliases.
param([string]$Data = "data")
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::new()
$env:PYTHONUTF8 = "1"
$env:PYTHONIOENCODING = "utf-8"
$Python = $env:BIZSTUDIO_PYTHON
if (-not $Python) { Write-Error "Cài Python trong Thiết lập đầy đủ rồi thử lại."; exit 1 }
& $Python -I -c "import sys, venv; assert sys.version_info >= (3, 10) and sys.maxsize > 2**32"
if ($LASTEXITCODE -ne 0) { Write-Error "Python không tương thích"; exit 1 }
$Venv = Join-Path $Data "agent-sdk\venv"
New-Item -ItemType Directory -Force -Path (Join-Path $Data "agent-sdk") | Out-Null
& $Python -m venv $Venv
if ($LASTEXITCODE -ne 0) { Write-Error "Tạo venv SDK thất bại"; exit 1 }
$VenvPy = Join-Path $Venv "Scripts\python.exe"
& $VenvPy -m pip install --disable-pip-version-check "claude-agent-sdk==0.2.152"
if ($LASTEXITCODE -ne 0) { Write-Error "Cài SDK thất bại; kiểm tra mạng/proxy rồi thử lại"; exit 1 }
& $VenvPy -I -c "import claude_agent_sdk, importlib.metadata as m; assert m.version('claude-agent-sdk') == '0.2.152'; print('Claude Agent SDK đã cài. Chưa gọi API, chưa phát sinh phí.')"
if ($LASTEXITCODE -ne 0) { Write-Error "SDK chưa import được"; exit 1 }
exit 0
