#!/usr/bin/env bash
# Cài VieNeu-TTS (giọng đọc tiếng Việt tự nhiên, on-device, 48 kHz) cho Biz Studio.
# Dùng: ./scripts/setup-vieneu.sh [thư-mục-data]   (mặc định: data)
set -euo pipefail
cd "$(dirname "$0")/.."

DATA="${1:-data}"
VENV="$DATA/vieneu/venv"

echo "🦜 Cài VieNeu-TTS vào $VENV …"
mkdir -p "$DATA/vieneu"

if ! command -v python3 >/dev/null 2>&1; then
  echo "❌ Cần Python 3.10+ — cài tại https://www.python.org/downloads/" >&2
  exit 1
fi

python3 -m venv "$VENV"
"$VENV/bin/pip" install --quiet --upgrade pip
echo "→ pip install vieneu (CPU/ONNX, không cần GPU)…"
"$VENV/bin/pip" install vieneu

# torch/torchaudio chỉ cần cho Clone voice (trích đặc trưng giọng từ clip mẫu).
# Bỏ qua bằng: SKIP_CLONE=1 ./scripts/setup-vieneu.sh
if [ "${SKIP_CLONE:-0}" != "1" ]; then
  echo "→ pip install torch torchaudio (cho tính năng Clone voice, ~300 MB)…"
  "$VENV/bin/pip" install torch torchaudio
else
  echo "→ Bỏ qua torch/torchaudio — Clone voice sẽ không dùng được."
fi

echo "→ Tải model lần đầu + xuất danh sách giọng (có thể mất vài phút)…"
DATA_DIR="$DATA" "$VENV/bin/python" - <<'EOF'
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
