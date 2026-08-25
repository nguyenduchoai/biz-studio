#!/usr/bin/env bash
# Cài VieNeu-TTS (giọng đọc tiếng Việt tự nhiên, on-device, 48 kHz) cho Biz Studio.
# Dùng: ./scripts/setup-vieneu.sh [thư-mục-data]   (mặc định: data)
set -euo pipefail
# Không cd theo vị trí file: script này còn được nhúng vào binary và chạy từ
# thư mục tạm. Thư mục data truyền qua tham số 1 (tuyệt đối khi gọi từ app).

DATA="${1:-data}"
VENV="$DATA/vieneu/venv"

echo "🦜 Cài VieNeu-TTS vào $VENV …"
mkdir -p "$DATA/vieneu"

# Trên Apple Silicon, ép Python chạy arm64.
#
# Vì sao cần: nếu Biz Studio là bản x86_64 (hoặc terminal chạy dưới Rosetta),
# tiến trình con thừa hưởng x86_64 và pip tải về wheel x86_64. Nhưng lúc CHẠY,
# mã Go luôn ép python qua `arch -arm64` (xem internal/whisper, internal/tts) —
# lệch kiến trúc, và lỗi hiện ra là "incompatible architecture" ở tận bước nạp
# thư viện, chẳng dính gì tới việc cài. Ép ngay từ đây để hai bên luôn khớp.
#
# Dùng sysctl chứ không dùng `uname -m`: dưới Rosetta, uname nói dối là x86_64.
# Để là chuỗi (không phải mảng) vì bash 3.2 của macOS bung mảng rỗng dưới
# `set -u` là báo lỗi unbound variable.
ARCH_PREFIX=""
if [ "$(uname -s)" = "Darwin" ] && [ "$(sysctl -n hw.optional.arm64 2>/dev/null)" = "1" ] \
   && [ -x /usr/bin/arch ]; then
  ARCH_PREFIX="/usr/bin/arch -arm64"
  echo "→ Máy Apple Silicon — cài bản arm64 (nhanh hơn hẳn chạy qua Rosetta)."
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "❌ Cần Python 3.10+ — cài tại https://www.python.org/downloads/" >&2
  exit 1
fi

$ARCH_PREFIX python3 -m venv "$VENV"
$ARCH_PREFIX "$VENV/bin/pip" install --quiet --upgrade pip
echo "→ pip install vieneu 3.2.3 (bản đã kiểm với Biz Studio)…"
$ARCH_PREFIX "$VENV/bin/pip" install "vieneu==3.2.3"

# torch/torchaudio chỉ cần cho Clone voice (trích đặc trưng giọng từ clip mẫu).
# Bỏ qua bằng: SKIP_CLONE=1 ./scripts/setup-vieneu.sh
if [ "${SKIP_CLONE:-0}" != "1" ]; then
  echo "→ pip install torch torchaudio (cho tính năng Clone voice, ~300 MB)…"
  $ARCH_PREFIX "$VENV/bin/pip" install torch torchaudio
else
  echo "→ Bỏ qua torch/torchaudio — Clone voice sẽ không dùng được."
fi

echo "→ Tải model lần đầu + xuất danh sách giọng (có thể mất vài phút)…"
DATA_DIR="$DATA" $ARCH_PREFIX "$VENV/bin/python" - <<'EOF'
import json, os
from vieneu import Vieneu

v = Vieneu()
voices = [{"id": vid, "label": label} for label, vid in v.list_preset_voices()]
out = os.path.join(os.environ["DATA_DIR"], "vieneu", "voices.json")
with open(out, "w", encoding="utf-8") as f:
    json.dump(voices, f, ensure_ascii=False, indent=1)
print(f"✅ {len(voices)} giọng preset — đã ghi {out}")
EOF

echo
echo "✅ Xong! Mở Biz Studio → TTS / Giọng đọc: nhóm giọng VieNeu nằm đầu danh sách."
echo "   Engine giọng mặc định giờ tự ưu tiên VieNeu cho mọi tính năng (TTS, Vox, HTML Video)."
echo "   Clone voice: tab 🧬 trong trang TTS — tải lên clip mẫu 3–8 giây để nhân bản giọng."
