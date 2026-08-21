# Cài faster-whisper (bóc băng offline, có mốc TỪNG TỪ) cho Biz Studio.
# Dùng: powershell -ExecutionPolicy Bypass -File setup-whisper.ps1 [thư-mục-data]
#
# Biến môi trường:
#   WHISPER_MODEL=small   — model tải sẵn (tiny | base | small | medium | large-v3)
#   SKIP_MODEL=1          — bỏ qua bước tải model (tải lần đầu khi bóc băng)
param([string]$Data = "data")
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false   # tự kiểm tra $LASTEXITCODE

$Venv = Join-Path $Data "whisper\venv"
$Models = Join-Path $Data "whisper\models"
$Model = if ($env:WHISPER_MODEL) { $env:WHISPER_MODEL } else { "small" }

Write-Host "🎙  Cài faster-whisper vào $Venv …"
New-Item -ItemType Directory -Force -Path (Join-Path $Data "whisper") | Out-Null
New-Item -ItemType Directory -Force -Path $Models | Out-Null

# py launcher là cách chuẩn trên Windows; python.exe trong Store lắm khi chỉ là
# một cái stub mở Microsoft Store nên thử py trước.
$Py = $null
foreach ($c in @(@("py", "-3"), @("python", ""), @("python3", ""))) {
  $bin = $c[0]
  if (Get-Command $bin -ErrorAction SilentlyContinue) {
    $Py = $c
    break
  }
}
if (-not $Py) {
  Write-Error "❌ Cần Python 3.10+ — cài tại https://www.python.org/downloads/ (nhớ tích 'Add python.exe to PATH')"
  exit 1
}

$pyArgs = @()
if ($Py[1]) { $pyArgs += $Py[1] }
& $Py[0] @pyArgs -m venv $Venv
if ($LASTEXITCODE -ne 0) { Write-Error "❌ Tạo venv thất bại"; exit 1 }

$VenvPy = Join-Path $Venv "Scripts\python.exe"
$VenvPip = Join-Path $Venv "Scripts\pip.exe"

& $VenvPip install --quiet --upgrade pip
Write-Host "→ pip install faster-whisper (CPU/CTranslate2, không cần GPU)…"
& $VenvPip install faster-whisper
if ($LASTEXITCODE -ne 0) { Write-Error "❌ pip install faster-whisper thất bại"; exit 1 }

# Tải sẵn model để lần bóc băng đầu không phải chờ (small ~500 MB).
if ($env:SKIP_MODEL -ne "1") {
  Write-Host "→ Tải model `"$Model`" về $Models (lần đầu có thể mất vài phút)…"
  $snippet = @'
import os
from faster_whisper import WhisperModel

name = os.environ["WHISPER_MODEL"]
root = os.environ["MODELS_DIR"]
WhisperModel(name, device="auto", compute_type="auto", download_root=root)
print(f"✅ Model {name} đã sẵn sàng trong {root}")
'@
  $tmp = Join-Path $env:TEMP "bizstudio-whisper-model.py"
  Set-Content -Path $tmp -Value $snippet -Encoding UTF8
  $env:WHISPER_MODEL = $Model
  $env:MODELS_DIR = $Models
  & $VenvPy $tmp
  $code = $LASTEXITCODE
  Remove-Item $tmp -ErrorAction SilentlyContinue
  if ($code -ne 0) { Write-Error "❌ Tải model thất bại"; exit 1 }
} else {
  Write-Host "→ Bỏ qua tải model — lần bóc băng đầu tiên sẽ tự tải về $Models."
}

Write-Host ""
Write-Host "✅ Xong! Mở Biz Studio → OCR / ASR: chọn engine `"whisper`" để bóc băng offline."
Write-Host "   • Có mốc TỪNG TỪ → bật `"karaoke`" để xuất thêm file .ass tô sáng theo từng chữ."
Write-Host "   • File <tên>.words.json sinh kèm dùng cho Cắt khoảng lặng an toàn (transcript bảo vệ)."
Write-Host "   • Đổi model/compute trong Cấu hình & API (small nhanh, large-v3 chính xác nhất)."
