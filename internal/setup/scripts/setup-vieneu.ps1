# Cài VieNeu-TTS (giọng đọc tiếng Việt tự nhiên, on-device, 48 kHz) cho Biz Studio.
# Dùng: powershell -ExecutionPolicy Bypass -File setup-vieneu.ps1 [thư-mục-data]
#
# Biến môi trường:
#   SKIP_CLONE=1   — bỏ qua torch/torchaudio (~300 MB, chỉ cần cho Clone voice)
param([string]$Data = "data")
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false   # tự kiểm tra $LASTEXITCODE
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::new()
$env:PYTHONUTF8 = "1"
$env:PYTHONIOENCODING = "utf-8"
# Mạng chậm/rớt: pip chờ lâu hơn và tự thử lại; ưu tiên wheel để khỏi cần trình biên dịch C.
if (-not $env:PIP_TIMEOUT) { $env:PIP_TIMEOUT = "120" }
if (-not $env:PIP_RETRIES) { $env:PIP_RETRIES = "5" }
if (-not $env:PIP_PREFER_BINARY) { $env:PIP_PREFER_BINARY = "1" }
# Hugging Face: Windows 10, antivirus và proxy doanh nghiệp thường chặn Xet/symlink.
# Tải bằng HTTP chuẩn và cho mạng chậm thêm thời gian — cùng luật với setup-whisper.
$env:HF_HUB_DISABLE_XET = "1"
$env:HF_HUB_DISABLE_SYMLINKS_WARNING = "1"
$env:HF_HUB_ETAG_TIMEOUT = "30"
$env:HF_HUB_DOWNLOAD_TIMEOUT = "60"

$Venv = Join-Path $Data "vieneu\venv"
Write-Host "🦜 Cài VieNeu-TTS vào $Venv …"
New-Item -ItemType Directory -Force -Path (Join-Path $Data "vieneu") | Out-Null
# Windows giới hạn đường dẫn 260 ký tự; gói Python lồng sâu trong venv dễ vượt trần nếu
# thư mục dữ liệu đã dài. Chỉ cảnh báo — vẫn thử cài, nhưng người dùng biết vì sao hỏng.
$VenvAbs = [System.IO.Path]::GetFullPath($Venv)
if ($VenvAbs.Length -gt 120) {
  Write-Warning "Đường dẫn venv dài ($($VenvAbs.Length) ký tự): $VenvAbs. Nếu pip báo lỗi đường dẫn/không giải nén được, đổi thư mục dữ liệu ngắn hơn (VD: D:\BizStudio) rồi cài lại."
}

# Ưu tiên đúng Python 3.11 do bộ cài Full quản lý và loại Windows Store stub.
$Py = $null
$candidates = @(@("py", "-3.11"), @("py", "-3.12"), @("py", "-3.10"), @("py", "-3.13"), @("py", "-3"), @("python", ""), @("python3", ""))
if ($env:BIZSTUDIO_PYTHON) { $candidates = ,@($env:BIZSTUDIO_PYTHON, "") + $candidates }
foreach ($c in $candidates) {
  $bin = $c[0]
  if (-not (Get-Command $bin -ErrorAction SilentlyContinue)) { continue }
  $candidateArgs = @()
  if ($c[1]) { $candidateArgs += $c[1] }
  $valid = $false
  try {
    & $bin @candidateArgs -I -c "import sys, venv; raise SystemExit(0 if sys.version_info.major == 3 and 10 <= sys.version_info.minor <= 13 and sys.maxsize > 2**32 else 2)" 2>$null
    $valid = $LASTEXITCODE -eq 0
  } catch { $valid = $false }
  if ($valid) {
    $Py = $c
    break
  }
}
if (-not $Py) {
  Write-Error "❌ Cần Python 3.10–3.13 bản 64-bit — cài Python 3.11 tại https://www.python.org/downloads/"
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

Write-Host "→ Tải model lần đầu từ Hugging Face (~300 MB) + xuất danh sách giọng (mạng chậm có thể mất 10–20 phút)…"
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
if ($code -ne 0) {
  # Tải dở giữa chừng vì mạng rớt là chuyện thường; Hugging Face tải tiếp phần còn thiếu.
  Write-Warning "Tải model lần 1 chưa xong — thử lại một lần…"
  & $VenvPy $tmp
  $code = $LASTEXITCODE
}
Remove-Item $tmp -ErrorAction SilentlyContinue
if ($code -ne 0) { Write-Error "❌ Xuất danh sách giọng thất bại — kiểm tra mạng/proxy tới huggingface.co rồi bấm Cài lại (phần đã tải được giữ nguyên)"; exit 1 }

Write-Host ""
Write-Host "✅ Xong! Mở Biz Studio → TTS / Giọng đọc: nhóm giọng VieNeu nằm đầu danh sách."
Write-Host "   Engine giọng mặc định giờ tự ưu tiên VieNeu cho mọi tính năng (TTS, Vox, HTML Video)."
Write-Host "   Clone voice: tab 🧬 trong trang TTS — tải lên clip mẫu 3–8 giây để nhân bản giọng."
