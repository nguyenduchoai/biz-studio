# Cài faster-whisper (bóc băng offline, có mốc TỪNG TỪ) cho Biz Studio.
# Dùng: powershell -ExecutionPolicy Bypass -File setup-whisper.ps1 [thư-mục-data]
#
# Biến môi trường:
#   WHISPER_MODEL=small   — model tải sẵn (tiny | base | small | medium | large-v3)
#   SKIP_MODEL=1          — bỏ qua bước tải model (tải lần đầu khi bóc băng)
param([string]$Data = "data")
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false   # tự kiểm tra $LASTEXITCODE
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::new()

$Venv = Join-Path $Data "whisper\venv"
$Models = Join-Path $Data "whisper\models"
$Model = if ($env:WHISPER_MODEL) { $env:WHISPER_MODEL } else { "small" }

Write-Host "🎙  Cài faster-whisper vào $Venv …"
New-Item -ItemType Directory -Force -Path (Join-Path $Data "whisper") | Out-Null
New-Item -ItemType Directory -Force -Path $Models | Out-Null

# Ưu tiên đúng Python 3.11 do bộ cài Full quản lý. Không chỉ kiểm tra tên lệnh:
# WindowsApps có thể chứa python.exe giả chỉ để mở Microsoft Store.
$Py = $null
foreach ($c in @(@("py", "-3.11"), @("py", "-3"), @("python", ""), @("python3", ""))) {
  $bin = $c[0]
  if (-not (Get-Command $bin -ErrorAction SilentlyContinue)) { continue }
  $candidateArgs = @()
  if ($c[1]) { $candidateArgs += $c[1] }
  & $bin @candidateArgs -c "import sys; raise SystemExit(0 if sys.version_info >= (3, 10) and sys.maxsize > 2**32 else 2)" 2>$null
  if ($LASTEXITCODE -eq 0) {
    $Py = $c
    break
  }
}
if (-not $Py) {
  Write-Error "❌ Cần Python 3.10+ 64-bit — cài tại https://www.python.org/downloads/ (nhớ tích 'Add python.exe to PATH')"
  exit 1
}

$pyArgs = @()
if ($Py[1]) { $pyArgs += $Py[1] }
$PythonInfo = & $Py[0] @pyArgs -c "import sys; print(sys.executable + ' | Python ' + '.'.join(map(str, sys.version_info[:3])))"
Write-Host "✓ Đã nhận Python: $PythonInfo"
& $Py[0] @pyArgs -m venv $Venv
if ($LASTEXITCODE -ne 0) { Write-Error "❌ Tạo venv thất bại"; exit 1 }

$VenvPy = Join-Path $Venv "Scripts\python.exe"

& $VenvPy -m pip install --quiet --upgrade pip
if ($LASTEXITCODE -ne 0) { Write-Warning "Không nâng được pip; tiếp tục dùng pip hiện có trong venv." }
Write-Host "→ pip install faster-whisper 1.2.1 (bản cố định của Biz Studio)…"
& $VenvPy -m pip install "faster-whisper==1.2.1"
if ($LASTEXITCODE -ne 0) { Write-Error "❌ pip install faster-whisper thất bại"; exit 1 }
& $VenvPy -c "import faster_whisper, importlib.metadata as m; print('✓ faster-whisper ' + m.version('faster-whisper') + ' đã import được')"
if ($LASTEXITCODE -ne 0) { Write-Error "❌ Đã cài nhưng Python không import được faster-whisper"; exit 1 }

# Tải sẵn model để lần bóc băng đầu không phải chờ (small ~500 MB).
if ($env:SKIP_MODEL -ne "1") {
  Write-Host "→ Tải model `"$Model`" về $Models (lần đầu có thể mất vài phút)…"
  $snippet = @'
import os, sys
from faster_whisper.utils import download_model

name = os.environ["WHISPER_MODEL"]
root = os.environ["MODELS_DIR"]
try:
    path = download_model(name, cache_dir=root)
    print(f"✅ Model {name} đã sẵn sàng trong {path}")
except Exception as exc:
    print(f"⚠️ Chưa tải trước được model {name}: {type(exc).__name__}: {exc}", file=sys.stderr)
    raise
'@
  $tmp = Join-Path $env:TEMP ("bizstudio-whisper-model-" + [guid]::NewGuid().ToString("N") + ".py")
  Set-Content -Path $tmp -Value $snippet -Encoding UTF8
  $env:WHISPER_MODEL = $Model
  $env:MODELS_DIR = $Models
  # Xet/symlink thường bị proxy, antivirus hoặc quyền Windows 10 chặn. Hub có
  # đường HTTP chuẩn ổn định hơn; tăng timeout cho mạng chậm.
  $env:HF_HUB_DISABLE_XET = "1"
  $env:HF_HUB_DISABLE_SYMLINKS_WARNING = "1"
  $env:HF_HUB_ETAG_TIMEOUT = "30"
  $env:HF_HUB_DOWNLOAD_TIMEOUT = "60"
  & $VenvPy $tmp
  $code = $LASTEXITCODE
  Remove-Item $tmp -ErrorAction SilentlyContinue
  if ($code -ne 0) {
    Write-Warning "faster-whisper đã cài xong; model sẽ tải khi bóc băng lần đầu. Kiểm tra mạng/proxy nếu lượt đó vẫn lỗi."
  }
} else {
  Write-Host "→ Bỏ qua tải model — lần bóc băng đầu tiên sẽ tự tải về $Models."
}

Write-Host ""
Write-Host "✅ Xong! Mở Biz Studio → OCR / ASR: chọn engine `"whisper`" để bóc băng offline."
Write-Host "   • Có mốc TỪNG TỪ → bật `"karaoke`" để xuất thêm file .ass tô sáng theo từng chữ."
Write-Host "   • File <tên>.words.json sinh kèm dùng cho Cắt khoảng lặng an toàn (transcript bảo vệ)."
Write-Host "   • Đổi model/compute trong Cấu hình & API (small nhanh, large-v3 chính xác nhất)."
exit 0
