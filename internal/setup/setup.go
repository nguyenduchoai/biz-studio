// Package setup cài/cập nhật các công cụ ngoài mà Biz Studio phụ thuộc.
//
// Vì sao cần: người dùng gặp lỗi kiểu "HTTP Error 403: Forbidden" khi tải video
// và không có cách nào đoán ra nguyên nhân thật là yt-dlp cũ 6 tuần. Bắt họ mở
// terminal gõ brew/winget là mất luôn phần lớn người dùng. Nút "Cài" / "Cập
// nhật" ngay cạnh dòng trạng thái giải quyết đúng chỗ đó.
//
// Nguyên tắc: KHÔNG tải script lạ từ mạng về chạy. Chỉ gọi trình quản lý gói
// của hệ điều hành (brew / winget / apt…) và các script cài đặt nhúng sẵn
// trong binary — thứ người dùng đã tin khi tải chính phần mềm này.
package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Step là một lệnh trong quy trình cài. Nhiều bước chạy tuần tự, bước nào lỗi
// thì dừng — không chạy tiếp để tránh che mất lỗi gốc.
type Step struct {
	Label string
	Bin   string
	Args  []string
	Env   []string // thêm vào môi trường hiện tại, dạng "K=V"
}

// Plan là toàn bộ việc cần làm để cài (hoặc cập nhật) một công cụ trên MÁY NÀY.
// Dựng plan tách khỏi lúc chạy để giao diện xem trước được sẽ chạy lệnh gì.
type Plan struct {
	Tool    string   `json:"tool"`
	Action  string   `json:"action"` // "install" | "update"
	Steps   []Step   `json:"-"`
	Cmds    []string `json:"cmds"`   // dạng chữ để hiện cho người dùng
	Cleanup []string `json:"-"`      // file tạm cần xoá sau khi chạy
	Manual  string   `json:"manual"` // hướng dẫn tay khi không tự cài được
}

// BuildPlan dựng quy trình cài/cập nhật cho máy đang chạy.
//
// tmpDir dùng để ghi script nhúng ra đĩa (script không chạy được từ trong
// binary). dataDir là thư mục data của studio, truyền vào script làm tham số 1.
func BuildPlan(t Tool, action, dataDir, tmpDir string) (*Plan, error) {
	if action != "install" && action != "update" {
		return nil, fmt.Errorf("hành động không hợp lệ: %q", action)
	}
	p := &Plan{Tool: t.ID, Action: action, Manual: t.Manual}

	if t.script != "" {
		st, cleanup, err := scriptStep(t, action, dataDir, tmpDir)
		if err != nil {
			return nil, err
		}
		p.Steps = []Step{st}
		p.Cleanup = cleanup
	} else {
		st, err := pkgStep(t, action)
		if err != nil {
			return nil, err
		}
		p.Steps = []Step{st}
	}

	for _, s := range p.Steps {
		p.Cmds = append(p.Cmds, strings.TrimSpace(s.Bin+" "+strings.Join(s.Args, " ")))
	}
	return p, nil
}

// scriptStep ghi script nhúng ra file tạm rồi trả lệnh chạy nó.
//
// Cài lại và cập nhật là cùng một việc với script venv (pip install ghi đè), nên
// không phân biệt action ở đây.
func scriptStep(t Tool, _, dataDir, tmpDir string) (Step, []string, error) {
	ext, bin, pre := ".sh", "bash", []string(nil)
	if runtime.GOOS == "windows" {
		ext = ".ps1"
		systemRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
		if systemRoot == "" {
			systemRoot = strings.TrimSpace(os.Getenv("WINDIR"))
		}
		bin = filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(bin); err != nil {
			return Step{}, nil, fmt.Errorf("không tìm thấy Windows PowerShell hệ thống tại %s: %w", bin, err)
		}
		pre = []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File"}
	}
	name := t.script + ext
	body, err := scriptFile(name)
	if err != nil {
		return Step{}, nil, err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return Step{}, nil, fmt.Errorf("tạo thư mục tạm: %w", err)
	}
	if runtime.GOOS == "windows" {
		// Windows PowerShell 5.1 chỉ nhận UTF-8 chắc chắn khi file có BOM. Hai
		// script có tiếng Việt; thiếu BOM làm log và đôi khi literal bị mojibake.
		body = append([]byte{0xEF, 0xBB, 0xBF}, body...)
	}
	f, err := os.CreateTemp(tmpDir, t.script+"-*"+ext)
	if err != nil {
		return Step{}, nil, fmt.Errorf("tạo script cài đặt tạm: %w", err)
	}
	path := f.Name()
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return Step{}, nil, fmt.Errorf("ghi script cài đặt: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return Step{}, nil, fmt.Errorf("đóng script cài đặt: %w", err)
	}
	_ = os.Chmod(path, 0o700)
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		abs = dataDir
	}
	return Step{
		Label: "Chạy " + name,
		Bin:   bin,
		Args:  append(append([]string{}, pre...), path, abs),
	}, []string{path}, nil
}
