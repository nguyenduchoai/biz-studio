#!/usr/bin/env bash
# Đóng gói Biz Studio đa nền tảng: dmg (macOS), zip (Windows), tar.gz (Linux).
# Dùng: ./scripts/build-release.sh [version]   (mặc định 1.0.0)
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-1.0.0}"
LDFLAGS="-s -w"
rm -rf dist
mkdir -p dist

gobuild() { # gobuild <GOOS> <GOARCH> <out>
  echo "→ build $1/$2"
  GOOS="$1" GOARCH="$2" CGO_ENABLED=0 \
    go build -trimpath -ldflags "$LDFLAGS" -o "$3" ./cmd/bizstudio
}

# ---------- Windows (.exe trong zip) ----------
mkdir -p dist/win
gobuild windows amd64 dist/win/bizstudio.exe
cat > dist/win/HUONG-DAN.txt <<'EOF'
Biz Studio — chạy bizstudio.exe rồi mở http://localhost:6868
Yêu cầu: ffmpeg trong PATH (https://ffmpeg.org). Tùy chọn: Claude CLI, yt-dlp.
Dữ liệu lưu tại thư mục data\ cạnh file exe. Đổi cổng: bizstudio.exe -port 8080
EOF
(cd dist/win && zip -q -r "../BizStudio-windows-amd64.zip" .)

# ---------- Linux (tar.gz) ----------
for arch in amd64 arm64; do
  mkdir -p "dist/linux-$arch"
  gobuild linux "$arch" "dist/linux-$arch/bizstudio"
  tar -czf "dist/BizStudio-linux-$arch.tar.gz" -C "dist/linux-$arch" bizstudio
done

# ---------- macOS (.app trong .dmg) ----------
make_app() { # make_app <arch>
  local arch="$1"
  local appdir="dist/mac-$arch/Biz Studio.app"
  mkdir -p "$appdir/Contents/MacOS"
  gobuild darwin "$arch" "$appdir/Contents/MacOS/bizstudio"

  cat > "$appdir/Contents/MacOS/launcher" <<'EOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$HOME/Library/Application Support/BizStudio"
mkdir -p "$DATA"
if ! curl -sf -o /dev/null "http://localhost:6868/api/state" 2>/dev/null; then
  ( sleep 1.5; open "http://localhost:6868" ) &
  exec "$DIR/bizstudio" -port 6868 -data "$DATA"
else
  open "http://localhost:6868"
fi
EOF
  chmod +x "$appdir/Contents/MacOS/launcher"

  cat > "$appdir/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>Biz Studio</string>
  <key>CFBundleDisplayName</key><string>Biz Studio</string>
  <key>CFBundleIdentifier</key><string>com.hoainguyen.bizstudio</string>
  <key>CFBundleVersion</key><string>${VERSION}</string>
  <key>CFBundleShortVersionString</key><string>${VERSION}</string>
  <key>CFBundleExecutable</key><string>launcher</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>LSUIElement</key><true/>
  <key>NSHighResolutionCapable</key><true/>
</dict></plist>
EOF

  if command -v hdiutil >/dev/null 2>&1; then
    hdiutil create -quiet -volname "Biz Studio" \
      -srcfolder "dist/mac-$arch" -ov -format UDZO \
      "dist/BizStudio-macos-$arch.dmg"
  else
    echo "  (bỏ qua .dmg — hdiutil chỉ có trên macOS)"
    tar -czf "dist/BizStudio-macos-$arch.tar.gz" -C "dist/mac-$arch" "Biz Studio.app"
  fi
}
make_app arm64
make_app amd64

# Dọn thư mục trung gian, giữ artifact
rm -rf dist/win dist/linux-amd64 dist/linux-arm64 dist/mac-arm64 dist/mac-amd64
echo
echo "✅ Xong — artifact trong dist/:"
ls -lh dist/
