#!/bin/bash
set -eu
# Finder does not need an interactive/login shell. The executable discovers
# installed tools itself; user shell startup files must not block app startup.
launcher_dir="$(cd "$(dirname "$0")" && pwd)"
exec "$launcher_dir/bizstudio" -data "$HOME/Library/Application Support/BizStudio" "$@"
