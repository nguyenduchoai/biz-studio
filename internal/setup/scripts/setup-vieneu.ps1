# Cài VieNeu-TTS (giọng đọc tiếng Việt tự nhiên, on-device, 48 kHz) cho Biz Studio.
# Dùng: powershell -ExecutionPolicy Bypass -File setup-vieneu.ps1 [thư-mục-data]
#
# Biến môi trường:
#   SKIP_CLONE=1   — bỏ qua torch/torchaudio (~300 MB, chỉ cần cho Clone voice)
param([string]$Data = "data")
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false   # tự kiểm tra $LASTEXITCODE

$Venv = Join-Path $Data "vieneu\venv"
Write-Host "🦜 Cài VieNeu-TTS vào $Venv …"
New-Item -ItemType Directory -Force -Path (Join-Path $Data "vieneu") | Out-Null

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
Write-Host "→ pip install vieneu (CPU/ONNX, không cần GPU)…"
& $VenvPip install vieneu
if ($LASTEXITCODE -ne 0) { Write-Error "❌ pip install vieneu thất bại"; exit 1 }

# torch/torchaudio chỉ cần cho Clone voice (trích đặc trưng giọng từ clip mẫu).
if ($env:SKIP_CLONE -ne "1") {
  Write-Host "→ pip install torch torchaudio (cho tính năng Clone voice, ~300 MB)…"
  & $VenvPip install torch torchaudio
  if ($LASTEXITCODE -ne 0) { Write-Error "❌ pip install torch thất bại"; exit 1 }
} else {
  Write-Host "→ Bỏ qua torch/torchaudio — Clone voice sẽ không dùng được."
}

Write-Host "→ Tải model lần đầu + xuất danh sách giọng (có thể mất vài phút)…"
$snippet = @'
import json, os
from vieneu import Vieneu

v = Vieneu()
voices = [{"id": vid, "label": label} for label, vid in v.list_preset_voices()]
out = os.path.join(os.environ["DATA_DIR"], "vieneu", "voices.json")
with open(out, "w", encoding="utf-8") as f:
    json.dump(voices, f, ensure_ascii=False, indent=1)
print(f"✅ {len(voices)} giọng preset — đã ghi {out}")
'@
$tmp = Join-Path $env:TEMP "bizstudio-vieneu-voices.py"
Set-Content -Path $tmp -Value $snippet -Encoding UTF8
$env:DATA_DIR = $Data
& $VenvPy $tmp
$code = $LASTEXITCODE
Remove-Item $tmp -ErrorAction SilentlyContinue
if ($code -ne 0) { Write-Error "❌ Xuất danh sách giọng thất bại"; exit 1 }

Write-Host ""
Write-Host "✅ Xong! Mở Biz Studio → TTS / Giọng đọc: nhóm giọng VieNeu nằm đầu danh sách."
Write-Host "   Engine giọng mặc định giờ tự ưu tiên VieNeu cho mọi tính năng (TTS, Vox, HTML Video)."
Write-Host "   Clone voice: tab 🧬 trong trang TTS — tải lên clip mẫu 3–8 giây để nhân bản giọng."
