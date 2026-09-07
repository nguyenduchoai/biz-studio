#!/usr/bin/env bash
# Optional SDK runtime; reuses the interpreter already verified by Full setup.
set -euo pipefail
DATA="${1:-data}"
VENV="$DATA/agent-sdk/venv"
PYTHON="${BIZSTUDIO_PYTHON:-}"
ARCH_PREFIX=""
if [ "$(uname -s)" = "Darwin" ] && [ "$(sysctl -n hw.optional.arm64 2>/dev/null)" = "1" ]; then
  ARCH_PREFIX="/usr/bin/arch -arm64"
fi
if [ -z "$PYTHON" ]; then
  echo "Cài Python trong Thiết lập đầy đủ rồi thử lại." >&2
  exit 1
fi
$ARCH_PREFIX "$PYTHON" -I -X utf8 -c 'import sys, venv; assert sys.version_info >= (3, 10) and sys.maxsize > 2**32'
mkdir -p "$DATA/agent-sdk"
$ARCH_PREFIX "$PYTHON" -X utf8 -m venv "$VENV"
$ARCH_PREFIX "$VENV/bin/python" -X utf8 -m pip install --disable-pip-version-check "claude-agent-sdk==0.2.152"
$ARCH_PREFIX "$VENV/bin/python" -I -X utf8 -c "import claude_agent_sdk, importlib.metadata as m; assert m.version('claude-agent-sdk') == '0.2.152'; print('Claude Agent SDK đã cài. Chưa gọi API, chưa phát sinh phí.')"
