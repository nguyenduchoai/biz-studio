package setup

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// pkgStep dựng lệnh cài/cập nhật bằng trình quản lý gói của hệ điều hành.
func pkgStep(t Tool, action string) (Step, error) {
	// Cập nhật thứ mà trình quản lý gói KHÔNG quản lý sẽ báo lỗi khó hiểu kiểu
	// "yt-dlp not installed" trong khi lệnh yt-dlp vẫn chạy được. Trường hợp đó
	// dùng lệnh tự cập nhật của chính công cụ.
	if action == "update" && len(t.selfUpdate) > 0 && !pkgOwns(t) {
		return Step{Label: strings.Join(t.selfUpdate, " "),
			Bin: t.selfUpdate[0], Args: t.selfUpdate[1:]}, nil
	}
	switch runtime.GOOS {
	case "darwin":
		return brewStep(t, action)
	case "windows":
		return wingetStep(t, action)
	default:
		return linuxStep(t, action)
	}
}

func brewStep(t Tool, action string) (Step, error) {
	if !have("brew") {
		return Step{}, fmt.Errorf("máy chưa có Homebrew — cài tại https://brew.sh rồi bấm lại, "+
			"hoặc tải thủ công tại %s", t.Manual)
	}
	if t.pkg.brewCask != "" {
		// Cask .app không cần sudo; cài lại đè lên bản cũ nên update dùng luôn install.
		return Step{Label: "brew " + t.pkg.brewCask, Bin: "brew",
			Args: []string{"install", "--cask", "--force", t.pkg.brewCask}}, nil
	}
	if t.pkg.brew == "" {
		return Step{}, fmt.Errorf("chưa hỗ trợ cài %s tự động trên macOS — xem %s", t.Label, t.Manual)
	}
	verb := "install"
	if action == "update" {
		verb = "upgrade"
	}
	return Step{Label: "brew " + verb + " " + t.pkg.brew, Bin: "brew", Args: []string{verb, t.pkg.brew}}, nil
}

func wingetStep(t Tool, action string) (Step, error) {
	if !have("winget") {
		return Step{}, fmt.Errorf("máy chưa có winget (App Installer) — cài từ Microsoft Store, "+
			"hoặc tải thủ công tại %s", t.Manual)
	}
	if t.pkg.winget == "" {
		return Step{}, fmt.Errorf("chưa hỗ trợ cài %s tự động trên Windows — xem %s", t.Label, t.Manual)
	}
	verb := "install"
	if action == "update" {
		verb = "upgrade"
	}
	// -e khớp đúng ID; hai cờ accept để không treo ở màn hình hỏi đồng ý.
	return Step{
		Label: "winget " + verb + " " + t.pkg.winget,
		Bin:   "winget",
		Args: []string{verb, "--id", t.pkg.winget, "-e", "--silent",
			"--accept-package-agreements", "--accept-source-agreements"},
	}, nil
}

// linuxStep dò apt/dnf/pacman. Cần quyền root: chỉ dùng sudo -n (không hỏi mật
// khẩu) — máy nào chưa cấu hình sẵn thì báo đúng câu lệnh để tự chạy, chứ không
// treo server chờ nhập mật khẩu vào khoảng không.
func linuxStep(t Tool, _ string) (Step, error) {
	mgrs := []struct {
		bin  string
		pkg  string
		args []string
	}{
		{"apt-get", t.pkg.apt, []string{"install", "-y"}},
		{"dnf", t.pkg.dnf, []string{"install", "-y"}},
		{"pacman", t.pkg.pacman, []string{"-S", "--noconfirm"}},
	}
	for _, m := range mgrs {
		if m.pkg == "" || !have(m.bin) {
			continue
		}
		full := append(append([]string{m.bin}, m.args...), m.pkg)
		if !sudoReady() {
			return Step{}, fmt.Errorf("cần quyền quản trị — mở terminal và chạy:\n\n    sudo %s",
				strings.Join(full, " "))
		}
		return Step{Label: strings.Join(full, " "), Bin: "sudo", Args: append([]string{"-n"}, full...)}, nil
	}
	return Step{}, fmt.Errorf("không nhận ra trình quản lý gói của bản Linux này — cài %s thủ công: %s",
		t.Label, t.Manual)
}

// sudoReady kiểm tra sudo có chạy được KHÔNG cần mật khẩu (NOPASSWD hoặc còn
// hạn phiên). Chạy `sudo -n true` là cách rẻ và không tác dụng phụ.
func sudoReady() bool {
	if !have("sudo") {
		return false
	}
	return exec.Command("sudo", "-n", "true").Run() == nil
}

// pkgOwns cho biết công cụ có phải do trình quản lý gói của máy cài hay không.
// Chỉ hỏi được ở macOS/Windows; Linux coi như có (apt/dnf vẫn nâng cấp được).
func pkgOwns(t Tool) bool {
	switch runtime.GOOS {
	case "darwin":
		if !have("brew") || t.pkg.brew == "" {
			return false
		}
		return exec.Command("brew", "list", "--formula", "--versions", t.pkg.brew).Run() == nil
	case "windows":
		if !have("winget") || t.pkg.winget == "" {
			return false
		}
		return exec.Command("winget", "list", "--id", t.pkg.winget, "-e").Run() == nil
	default:
		return true
	}
}

func have(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}
