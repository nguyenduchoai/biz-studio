package util

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultDataDir trả thư mục dữ liệu mặc định của cả GUI lẫn CLI. Trên
// Windows, bản nâng cấp vẫn ưu tiên data/db.json cạnh EXE nếu người dùng đã có
// dữ liệu từ các bản portable cũ; cài mới dùng LocalAppData để EXE có thể nằm
// trong Program Files mà không cần quyền ghi quản trị.
func DefaultDataDir() string {
	configDir, _ := os.UserConfigDir()
	executable, _ := os.Executable()
	return DefaultDataDirFor(runtime.GOOS, os.Getenv("LOCALAPPDATA"), configDir, executable)
}

func DefaultDataDirFor(goos, localAppData, configDir, executable string) string {
	if goos != "windows" {
		return "data"
	}
	if executable != "" {
		legacy := filepath.Join(filepath.Dir(executable), "data")
		if info, err := os.Stat(filepath.Join(legacy, "db.json")); err == nil && !info.IsDir() {
			return legacy
		}
	}
	base := strings.TrimSpace(localAppData)
	if base == "" {
		base = strings.TrimSpace(configDir)
	}
	if base == "" {
		return "data"
	}
	return filepath.Join(base, "BizStudio")
}

// DataDirID là định danh không tiết lộ đường dẫn dùng để chắc chắn một cửa sổ
// chỉ kết nối đúng tiến trình đang giữ cùng kho dữ liệu.
func DataDirID(dataDir string) string {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		abs = filepath.Clean(dataDir)
	}
	if runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:16])
}
