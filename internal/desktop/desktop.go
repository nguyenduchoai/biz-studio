// Package desktop mở giao diện Biz Studio trong một CỬA SỔ APP riêng thay vì
// một tab trình duyệt.
//
// Cách làm: mọi trình duyệt họ Chromium đều có cờ `--app=<url>` — cửa sổ không
// thanh địa chỉ, không tab, không nút back, có mục riêng trên Dock/taskbar.
// Người dùng nhìn vào không phân biệt được với app thường.
//
// Vì sao không dùng webview của hệ điều hành: webview bắt buộc CGO, mà bật CGO
// là mất khả năng cross-compile cả 5 nền tảng từ một máy Mac — phải dựng CI
// riêng cho từng hệ điều hành. Cái giá đó quá lớn so với thứ nhận lại, nhất là
// khi Chrome vốn đã là điều kiện bắt buộc của HTML Video và Edge thì máy
// Windows 10/11 nào cũng có sẵn.
package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"bizstudio/internal/htmlvideo"
	"bizstudio/internal/store"
)

// appWindowSize — kích thước cửa sổ mở lần đầu. Sau đó Chromium tự nhớ kích
// thước người dùng chỉnh, nhờ có user-data-dir riêng.
const appWindowSize = "1440,900"

// chromiumFamily — các trình duyệt họ Chromium hiểu cờ --app, xếp theo thứ tự
// ưu tiên. Rộng hơn danh sách của HTML Video: render video cần đúng
// Chrome/Chromium để khung hình ổn định, còn mở một cửa sổ thì Edge hay Brave
// đều làm được như nhau.
func chromiumFamily() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",
			"/Applications/CocCoc.app/Contents/MacOS/CocCoc",
		}
	case "windows":
		var out []string
		for _, base := range []string{
			os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData"),
		} {
			if base == "" {
				continue
			}
			out = append(out,
				filepath.Join(base, `Google\Chrome\Application\chrome.exe`),
				filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`),
				filepath.Join(base, `BraveSoftware\Brave-Browser\Application\brave.exe`),
			)
		}
		return out
	default:
		return []string{
			"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
			"microsoft-edge", "brave-browser", "vivaldi-stable",
		}
	}
}

// FindBrowser dò trình duyệt mở được cửa sổ app.
//
// Hỏi htmlvideo.FindChrome trước để tôn trọng đường dẫn người dùng đã điền ở
// Cấu hình & API — có đúng một chỗ khai báo, không đẻ thêm bộ dò thứ ba.
func FindBrowser(st *store.Store) string {
	if bin, err := htmlvideo.FindChrome(st); err == nil && bin != "" {
		return bin
	}
	for _, c := range chromiumFamily() {
		if filepath.IsAbs(c) {
			if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
				return c
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// OpenWindow mở url trong cửa sổ app và trả tiến trình trình duyệt.
//
// Trả (nil, nil) khi máy không có trình duyệt họ Chromium: khi đó đã mở bằng
// trình duyệt mặc định rồi — thà một tab bình thường còn hơn màn hình trắng.
func OpenWindow(st *store.Store, url, dataDir string) (*exec.Cmd, error) {
	bin := FindBrowser(st)
	if bin == "" {
		return nil, OpenDefault(url)
	}

	// Hồ sơ riêng: không đụng vào phiên đăng nhập và tab đang mở của người dùng,
	// và là điều kiện để cửa sổ có mục riêng trên Dock/taskbar thay vì gộp
	// chung với trình duyệt đang chạy.
	profile := filepath.Join(dataDir, "appwindow")
	if err := os.MkdirAll(profile, 0o755); err != nil {
		return nil, fmt.Errorf("tạo hồ sơ cửa sổ app: %w", err)
	}

	args := []string{
		"--app=" + url,
		"--user-data-dir=" + profile,
		"--window-size=" + appWindowSize,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-features=Translate,MediaRouter",
	}
	if runtime.GOOS == "linux" {
		// WM_CLASS quyết định tên và icon hiện trên thanh tác vụ Linux.
		args = append(args, "--class=BizStudio")
	}

	cmd := exec.Command(bin, args...)
	// Trình duyệt in đủ thứ cảnh báo GPU/sandbox ra stderr — đổ đi, nếu không
	// nhật ký của studio bị lấp không đọc được.
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mở cửa sổ app bằng %s: %w", filepath.Base(bin), err)
	}
	return cmd, nil
}

// OpenDefault mở url bằng trình duyệt mặc định của hệ điều hành.
func OpenDefault(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		// rundll32 thay vì `cmd /c start`: start coi & trong URL là ký tự đặc
		// biệt của shell và cắt mất phần sau.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mở trình duyệt: %w", err)
	}
	// Không Wait: mấy lệnh này trả về ngay, giữ lại chỉ tổ đọng tiến trình zombie.
	go func() { _ = cmd.Wait() }()
	return nil
}

// HasDisplay cho biết máy có màn hình để mở cửa sổ hay không.
//
// Máy chủ Linux không màn hình mà cứ gọi trình duyệt thì nó treo vài giây rồi
// chết kèm một đống lỗi Xlib chẳng liên quan gì tới Biz Studio.
func HasDisplay() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" ||
		strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}
