#!/usr/bin/env bash
# Cài faster-whisper (bóc băng offline, có mốc TỪNG TỪ) cho Biz Studio.
# Dùng: ./scripts/setup-whisper.sh [thư-mục-data]   (mặc định: data)
#
# Biến môi trường:
#   WHISPER_MODEL=small   — model tải sẵn (tiny | base | small | medium | large-v3)
#   SKIP_MODEL=1          — bỏ qua bước tải model (tải lần đầu khi bóc băng)
set -euo pipefail
cd "$(dirname "$0")/.."

DATA="${1:-data}"
VENV="$DATA/whisper/venv"
MODELS="$DATA/whisper/models"
MODEL="${WHISPER_MODEL:-small}"

echo "🎙  Cài faster-whisper vào $VENV …"
mkdir -p "$DATA/whisper" "$MODELS"

if ! command -v python3 >/dev/null 2>&1; then
  echo "❌ Cần Python 3.10+ — cài tại https://www.python.org/downloads/" >&2
  exit 1
fi

python3 -m venv "$VENV"
"$VENV/bin/pip" install --quiet --upgrade pip
echo "→ pip install faster-whisper (CPU/CTranslate2, không cần GPU)…"
"$VENV/bin/pip" install faster-whisper

# Tải sẵn model để lần bóc băng đầu không phải chờ (small ~500 MB).
# Bỏ qua bằng: SKIP_MODEL=1 ./scripts/setup-whisper.sh
if [ "${SKIP_MODEL:-0}" != "1" ]; then
  echo "→ Tải model \"$MODEL\" về $MODELS (lần đầu có thể mất vài phút)…"
  WHISPER_MODEL="$MODEL" MODELS_DIR="$MODELS" "$VENV/bin/python" - <<'EOF'
import os
from faster_whisper import WhisperModel

name = os.environ["WHISPER_MODEL"]
root = os.environ["MODELS_DIR"]
WhisperModel(name, device="auto", compute_type="auto", download_root=root)
print(f"✅ Model {name} đã sẵn sàng trong {root}")
EOF
else
  echo "→ Bỏ qua tải model — lần bóc băng đầu tiên sẽ tự tải về $MODELS."
fi

echo
echo "✅ Xong! Mở Biz Studio → OCR / ASR: chọn engine \"whisper\" để bóc băng offline."
echo "   • Có mốc TỪNG TỪ → bật \"karaoke\" để xuất thêm file .ass tô sáng theo từng chữ."
echo "   • File <tên>.words.json sinh kèm dùng cho Cắt khoảng lặng an toàn (transcript bảo vệ)."
echo "   • Đổi model/compute trong Cấu hình & API (small nhanh, large-v3 chính xác nhất)."
