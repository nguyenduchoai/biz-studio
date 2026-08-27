# Cài VieNeu-TTS (giọng đọc tiếng Việt tự nhiên, on-device, 48 kHz) cho Biz Studio.
# Dùng: powershell -ExecutionPolicy Bypass -File setup-vieneu.ps1 [thư-mục-data]
#
# Biến môi trường:
#   SKIP_CLONE=1   — bỏ qua torch/torchaudio (~300 MB, chỉ cần cho Clone voice)
param([string]$Data = "data")
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false   # tự kiểm tra $LASTEXITCODE
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::new()

$Venv = Join-Path $Data "vieneu\venv"
Write-Host "🦜 Cài VieNeu-TTS vào $Venv …"
New-Item -ItemType Directory -Force -Path (Join-Path $Data "vieneu") | Out-Null

# Ưu tiên đúng Python 3.11 do bộ cài Full quản lý và loại Windows Store stub.
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
Write-Host "→ pip install vieneu 3.2.3 (bản đã kiểm với Biz Studio)…"
& $VenvPy -m pip install "vieneu==3.2.3"
if ($LASTEXITCODE -ne 0) { Write-Error "❌ pip install vieneu thất bại"; exit 1 }

# torch/torchaudio chỉ cần cho Clone voice (trích đặc trưng giọng từ clip mẫu).
if ($env:SKIP_CLONE -ne "1") {
  Write-Host "→ pip install torch torchaudio (cho tính năng Clone voice, ~300 MB)…"
  & $VenvPy -m pip install torch torchaudio
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
$tmp = Join-Path $env:TEMP ("bizstudio-vieneu-voices-" + [guid]::NewGuid().ToString("N") + ".py")
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
