#!/usr/bin/env bash
# Đóng gói Biz Studio đa nền tảng: dmg (macOS), zip (Windows), tar.gz (Linux).
# Dùng: ./scripts/build-release.sh <version>
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "Thiếu version. Dùng: ./scripts/build-release.sh 2.14.0" >&2
  exit 2
fi
if [ "${ALLOW_DIRTY:-0}" != "1" ] && [ -n "$(git status --porcelain)" ]; then
  echo "Worktree đang có thay đổi. Commit/test xong trước khi build release; chỉ RC nội bộ mới dùng ALLOW_DIRTY=1." >&2
  exit 2
fi
LDFLAGS="-s -w -X bizstudio/internal/server.Version=${VERSION}"
rm -rf dist
mkdir -p dist

gobuild() { # gobuild <GOOS> <GOARCH> <out>
  echo "→ build $1/$2"
  GOOS="$1" GOARCH="$2" CGO_ENABLED=0 \
    go build -trimpath -ldflags "$LDFLAGS" -o "$3" ./cmd/bizstudio
}

# ---------- Windows (.exe trong zip) ----------
#
# Hai file exe, cố ý:
#   "Biz Studio.exe"  build với -H windowsgui → bấm đúp là ra cửa sổ app, KHÔNG
#                     kèm cửa sổ console đen thui bên cạnh.
#   bizstudio.exe     build thường → dùng cho dòng lệnh (bizstudio setup …).
# Không gộp được: -H windowsgui cắt luôn stdout nên lệnh CLI sẽ câm.
mkdir -p dist/win
gobuild windows amd64 dist/win/bizstudio.exe
echo "→ build windows/amd64 (bản cửa sổ app)"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "$LDFLAGS -H windowsgui" -o "dist/win/Biz Studio.exe" ./cmd/bizstudio
cat > dist/win/HUONG-DAN.txt <<'EOF'
Biz Studio
==========

Bấm đúp "Biz Studio.exe" — mở ra cửa sổ app, không cần trình duyệt.
(Cửa sổ dùng Chrome hoặc Microsoft Edge đã có sẵn trên máy. Windows 10/11
luôn có Edge nên không phải cài thêm gì.)

Đóng cửa sổ là thoát. Nếu còn việc đang render/cài đặt, máy chủ vẫn chạy tới khi xong.

Lần mở đầu:
  1. Xem danh sách và bấm "Cài đầy đủ thành phần còn thiếu".
  2. Chờ Git, Python, FFmpeg, yt-dlp, trình duyệt, Claude CLI, VieNeu và
     Whisper cài xong. Có thể mất lâu vì model giọng/bóc băng khá lớn.
  3. Mở PowerShell, chạy: claude auth login
     Sau khi đăng nhập xong, quay lại Biz Studio bấm "Kiểm tra lại".

Dòng lệnh — dùng bizstudio.exe (bản có console):
  bizstudio.exe setup                  liệt kê công cụ cài được
  bizstudio.exe setup yt-dlp --update  cập nhật yt-dlp (chữa lỗi 403 khi tải)
  bizstudio.exe -port 8080             đổi cổng
  bizstudio.exe -window=false          chỉ chạy máy chủ, không mở cửa sổ

Windows cần App Installer/WinGet (có sẵn trên Windows 10/11 cập nhật). Bộ cài
chỉ cài phần còn thiếu và hỏi xác nhận trước khi chạy.
Dữ liệu cài mới: %LOCALAPPDATA%\BizStudio
Nâng cấp portable: nếu data\db.json nằm cạnh EXE, Biz Studio giữ nguyên dữ liệu đó.
EOF
(cd dist/win && zip -q -r "../BizStudio-windows-amd64.zip" .)

# ---------- Linux (tar.gz) ----------
for arch in amd64 arm64; do
  mkdir -p "dist/linux-$arch"
  gobuild linux "$arch" "dist/linux-$arch/bizstudio"
  # .desktop để Biz Studio hiện trong menu ứng dụng như app thường.
  # StartupWMClass phải khớp --class mà internal/desktop truyền cho trình duyệt,
  # nếu không thanh tác vụ hiện icon Chrome thay vì mục Biz Studio.
  cat > "dist/linux-$arch/bizstudio.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=Biz Studio
Comment=Studio dựng video AI chạy trên máy
Exec=bizstudio
Terminal=false
Categories=AudioVideo;Video;
StartupWMClass=BizStudio
EOF
  cat > "dist/linux-$arch/HUONG-DAN.txt" <<'EOF'
Biz Studio
==========

  ./bizstudio                    mở cửa sổ app (cần Chrome/Chromium/Edge)
  ./bizstudio -window=false      chỉ chạy máy chủ (máy không màn hình)
  ./bizstudio setup              liệt kê công cụ cài được
  ./bizstudio setup yt-dlp --update

Hiện trong menu ứng dụng:
  cp bizstudio ~/.local/bin/
  cp bizstudio.desktop ~/.local/share/applications/
EOF
  tar -czf "dist/BizStudio-linux-$arch.tar.gz" -C "dist/linux-$arch" .
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
# Binary tự lo cả hai việc: mở cửa sổ app, và nếu đã có bản đang chạy thì mở
# thêm cửa sổ rồi thoát. Trước đây launcher tự curl rồi `open` — hai chỗ cùng
# quyết định một việc, sửa một chỗ là lệch.
# Chạy qua login shell để kế thừa PATH của người dùng (claude, ffmpeg, yt-dlp…)
exec /bin/zsh -l -c "exec \"$DIR/bizstudio\" -port 6868 -data \"$DATA\""
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

# Tạo sau CẢ Windows/Linux/macOS để manifest không bỏ sót DMG. Basename tương
# đối giúp `cd dist && shasum -a 256 -c SHA256SUMS.txt` chạy được ở máy nhận.
(cd dist && : > SHA256SUMS.txt && for artifact in ./*.zip ./*.tar.gz ./*.dmg; do
  [ -f "$artifact" ] && shasum -a 256 "$artifact" >> SHA256SUMS.txt
done)

# Dọn thư mục trung gian, giữ artifact
rm -rf dist/win dist/linux-amd64 dist/linux-arm64 dist/mac-arm64 dist/mac-amd64
echo
echo "✅ Xong — artifact trong dist/:"
ls -lh dist/
