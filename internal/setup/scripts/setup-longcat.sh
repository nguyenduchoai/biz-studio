#!/usr/bin/env bash
# Cài LongCat-Video-Avatar trên MÁY CÓ GPU NVIDIA.
#
# LongCat là model 13,6 tỉ tham số, bắt buộc CUDA — KHÔNG có bản cho Apple
# Silicon hay CPU. Chạy script này trên máy GPU (Linux + NVIDIA), rồi:
#   - Biz Studio chạy ngay trên máy đó  → Cấu hình & API chọn chế độ "local"
#   - Biz Studio chạy ở máy khác        → chạy scripts/longcat-worker.py ở đây,
#                                          máy kia chọn chế độ "remote"
set -euo pipefail
# Không cd theo vị trí file: script này còn được nhúng vào binary và chạy từ
# thư mục tạm. Thư mục data truyền qua tham số 1 (tuyệt đối khi gọi từ app).
ROOT="${LONGCAT_DIR:-$PWD/data/longcat}"
SKIP_WEIGHTS="${SKIP_WEIGHTS:-0}"

command -v nvidia-smi >/dev/null || {
  echo "✗ Không thấy nvidia-smi — máy này không có GPU NVIDIA."
  echo "  LongCat không chạy được ở đây. Cài trên máy có GPU rồi dùng chế độ remote."
  exit 1
}
echo "→ GPU: $(nvidia-smi --query-gpu=name --format=csv,noheader | paste -sd' + ' -)"

mkdir -p "$ROOT"
if [ ! -d "$ROOT/LongCat-Video/.git" ]; then
  echo "→ Tải mã nguồn LongCat-Video…"
  git clone --depth 1 https://github.com/meituan-longcat/LongCat-Video.git "$ROOT/LongCat-Video"
fi
REPO="$ROOT/LongCat-Video"

if [ ! -d "$REPO/venv" ]; then
  echo "→ Tạo môi trường Python…"
  python3 -m venv "$REPO/venv"
fi
PY="$REPO/venv/bin/python"

echo "→ Cài PyTorch (CUDA 12.4)…"
"$PY" -m pip install -q --upgrade pip
"$PY" -m pip install -q torch==2.6.0 torchvision==0.21.0 torchaudio==2.6.0 --index-url https://download.pytorch.org/whl/cu124
echo "→ Cài phụ thuộc…"
"$PY" -m pip install -q ninja psutil packaging
"$PY" -m pip install -q flash_attn==2.7.4.post1 || echo "  ⚠ flash_attn cài lỗi — có thể dùng xformers thay, xem README của repo"
[ -f "$REPO/requirements.txt" ] && "$PY" -m pip install -q -r "$REPO/requirements.txt"
[ -f "$REPO/requirements_avatar.txt" ] && "$PY" -m pip install -q -r "$REPO/requirements_avatar.txt"

WEIGHTS="$REPO/weights/LongCat-Video-Avatar-1.5"
if [ "$SKIP_WEIGHTS" = "1" ]; then
  echo "→ Bỏ qua tải trọng số (SKIP_WEIGHTS=1)"
else
  echo "→ Tải trọng số model (rất nặng, chờ lâu)…"
  "$PY" -m pip install -q "huggingface_hub[cli]"
  "$REPO/venv/bin/huggingface-cli" download meituan-longcat/LongCat-Video-Avatar-1.5 --local-dir "$WEIGHTS"
fi

cat <<EOF

✅ Xong.

Điền vào Cấu hình & API của Biz Studio:
  Thư mục mã nguồn : $REPO
  Thư mục trọng số : $WEIGHTS
  Python           : $PY

Nếu Biz Studio chạy ở MÁY KHÁC, bật xưởng render ở đây:
  $PY $PWD/scripts/longcat-worker.py \\
      --repo "$REPO" --checkpoint "$WEIGHTS" --port 7070 --gpus 1 --int8
rồi ở máy kia chọn chế độ "remote" và điền http://<ip-máy-này>:7070
EOF
