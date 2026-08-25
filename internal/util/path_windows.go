//go:build windows

package util

import (
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// systemPATHEntries đọc lại PATH đã được WinGet cập nhật trong registry. Một
// process đang chạy không tự nhận WM_SETTINGCHANGE, nên nếu bỏ bước này nút
// cài báo xong nhưng phải tắt/mở app mới tìm thấy binary.
func systemPATHEntries() []string {
	var out []string
	for _, item := range []struct {
		root registry.Key
		path string
	}{
		{registry.CURRENT_USER, `Environment`},
		{registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`},
	} {
		key, err := registry.OpenKey(item.root, item.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		value, _, err := key.GetStringValue("Path")
		_ = key.Close()
		if err != nil {
			continue
		}
		for _, entry := range strings.Split(os.ExpandEnv(value), string(os.PathListSeparator)) {
			if entry = strings.Trim(strings.TrimSpace(entry), `"`); entry != "" {
				out = append(out, entry)
			}
		}
	}
	return out
}
