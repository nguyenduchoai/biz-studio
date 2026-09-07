#!/usr/bin/env bash
# Cài faster-whisper (bóc băng offline, có mốc TỪNG TỪ) cho Biz Studio.
# Dùng: ./scripts/setup-whisper.sh [thư-mục-data]   (mặc định: data)
#
# Biến môi trường:
#   WHISPER_MODEL=small   — model tải sẵn (tiny | base | small | medium | large-v3)
#   SKIP_MODEL=1          — bỏ qua bước tải model (tải lần đầu khi bóc băng)
set -euo pipefail
# Không cd theo vị trí file: script này còn được nhúng vào binary và chạy từ
# thư mục tạm. Thư mục data truyền qua tham số 1 (tuyệt đối khi gọi từ app).

DATA="${1:-data}"
VENV="$DATA/whisper/venv"
MODELS="$DATA/whisper/models"
MODEL="${WHISPER_MODEL:-small}"

echo "🎙  Cài faster-whisper vào $VENV …"
mkdir -p "$DATA/whisper" "$MODELS"

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

PYTHON="${BIZSTUDIO_PYTHON:-}"
PYTHON_CHECK='import sys, venv; raise SystemExit(0 if sys.version_info.major == 3 and 10 <= sys.version_info.minor <= 13 and sys.maxsize > 2**32 else 2)'
if [ -z "$PYTHON" ]; then
  for candidate in python3.11 /opt/homebrew/opt/python@3.11/bin/python3.11 /usr/local/opt/python@3.11/bin/python3.11 python3.12 python3.10 python3.13 python3 python; do
    if command -v "$candidate" >/dev/null 2>&1 && $ARCH_PREFIX "$candidate" -I -c "$PYTHON_CHECK" >/dev/null 2>&1; then
      PYTHON="$(command -v "$candidate")"
      break
    fi
  done
fi
if [ -z "$PYTHON" ] || ! $ARCH_PREFIX "$PYTHON" -I -c "$PYTHON_CHECK" >/dev/null 2>&1; then
  echo "❌ Cần Python 3.10–3.13 bản 64-bit — cài Python 3.11 tại https://www.python.org/downloads/" >&2
  exit 1
fi

echo "✓ Đã nhận Python: $PYTHON"
$ARCH_PREFIX "$PYTHON" -m venv "$VENV"
if ! $ARCH_PREFIX "$VENV/bin/python" -m pip install --quiet --upgrade pip; then
  echo "⚠️ Không nâng được pip; tiếp tục dùng pip hiện có trong venv." >&2
fi
echo "→ pip install faster-whisper 1.2.1 (bản cố định của Biz Studio)…"
$ARCH_PREFIX "$VENV/bin/python" -m pip install "faster-whisper==1.2.1"
$ARCH_PREFIX "$VENV/bin/python" -c "import faster_whisper; print('✓ faster-whisper đã import được')"

# Tải sẵn model để lần bóc băng đầu không phải chờ (small ~500 MB).
# Bỏ qua bằng: SKIP_MODEL=1 ./scripts/setup-whisper.sh
if [ "${SKIP_MODEL:-0}" != "1" ]; then
  echo "→ Tải model \"$MODEL\" về $MODELS (lần đầu có thể mất vài phút)…"
  if ! WHISPER_MODEL="$MODEL" MODELS_DIR="$MODELS" HF_HUB_ETAG_TIMEOUT=30 HF_HUB_DOWNLOAD_TIMEOUT=60 $ARCH_PREFIX "$VENV/bin/python" - <<'EOF'
import os
from faster_whisper.utils import download_model

name = os.environ["WHISPER_MODEL"]
root = os.environ["MODELS_DIR"]
download_model(name, cache_dir=root)
print(f"✅ Model {name} đã sẵn sàng trong {root}")
EOF
  then
    echo "⚠️ faster-whisper đã cài xong; model sẽ tải khi bóc băng lần đầu. Kiểm tra mạng/proxy nếu lượt đó vẫn lỗi." >&2
  fi
else
  echo "→ Bỏ qua tải model — lần bóc băng đầu tiên sẽ tự tải về $MODELS."
fi

echo
echo "✅ Xong! Mở Biz Studio → OCR / ASR: chọn engine \"whisper\" để bóc băng offline."
echo "   • Có mốc TỪNG TỪ → bật \"karaoke\" để xuất thêm file .ass tô sáng theo từng chữ."
echo "   • File <tên>.words.json sinh kèm dùng cho Cắt khoảng lặng an toàn (transcript bảo vệ)."
echo "   • Đổi model/compute trong Cấu hình & API (small nhanh, large-v3 chính xác nhất)."
