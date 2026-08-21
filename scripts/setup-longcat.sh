#!/usr/bin/env bash
# Vỏ bọc giữ nguyên lệnh quen thuộc ./scripts/setup-longcat.sh — bản thật nằm ở
# internal/setup/scripts/ để nhúng được vào binary (nút "Cài" trong Cấu hình
# dùng chính file đó, không có bản sao thứ hai).
set -euo pipefail
cd "$(dirname "$0")/.."
exec bash internal/setup/scripts/setup-longcat.sh "$@"
