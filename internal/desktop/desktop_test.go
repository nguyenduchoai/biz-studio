package desktop

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// Cửa sổ app chỉ "trông như app" nhờ đúng vài cờ này. Thiếu --app là ra tab
// trình duyệt đầy đủ thanh địa chỉ — hỏng đúng cái người dùng yêu cầu, mà lại
// hỏng lặng lẽ vì mọi thứ khác vẫn chạy.
func TestAppWindowFlagsAreComplete(t *testing.T) {
	// Dựng lại đúng bộ cờ OpenWindow truyền đi. Giữ đồng bộ bằng test này thay
	// vì tin vào trí nhớ.
	must := []string{
		"--app=",                     // không có: ra tab thường
		"--user-data-dir=",           // không có: đụng vào phiên đăng nhập của người dùng
		"--no-first-run",             // không có: hiện màn hình chào của trình duyệt
		"--no-default-browser-check", // không có: hỏi "đặt làm mặc định?"
	}
	src := readSource(t, "desktop.go")
	for _, flag := range must {
		if !strings.Contains(src, `"`+flag) {
			t.Errorf("thiếu cờ %q — cửa sổ sẽ không còn giống app", flag)
		}
	}
}

// Trình duyệt nào cũng phải là họ Chromium: Firefox/Safari không hiểu --app,
// đưa nhầm vào là mở ra một cửa sổ trình duyệt bình thường.
func TestBrowserCandidatesAreChromiumFamily(t *testing.T) {
	banned := []string{"firefox", "safari", "epiphany", "konqueror"}
	for _, c := range chromiumFamily() {
		low := strings.ToLower(c)
		for _, b := range banned {
			if strings.Contains(low, b) {
				t.Errorf("%q không thuộc họ Chromium — không hiểu cờ --app", c)
			}
		}
	}
}

func TestChromiumCandidatesNotEmpty(t *testing.T) {
	if len(chromiumFamily()) == 0 {
		t.Fatalf("không có ứng viên trình duyệt nào cho %s", runtime.GOOS)
	}
}

// Máy chủ Linux không màn hình mà gọi trình duyệt thì treo vài giây rồi chết
// kèm một đống lỗi Xlib chẳng liên quan gì tới Biz Studio.
func TestHasDisplay(t *testing.T) {
	if runtime.GOOS != "linux" {
		if !HasDisplay() {
			t.Errorf("HasDisplay() = false trên %s — chỉ Linux mới cần dò màn hình", runtime.GOOS)
		}
		return
	}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	if HasDisplay() {
		t.Error("HasDisplay() = true khi không có DISPLAY lẫn WAYLAND_DISPLAY")
	}
	t.Setenv("DISPLAY", ":0")
	if !HasDisplay() {
		t.Error("HasDisplay() = false khi DISPLAY=:0")
	}
}

func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("đọc %s: %v", name, err)
	}
	return string(b)
}
