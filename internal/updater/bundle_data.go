package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateDataOutsideBundle(stage Stage) error {
	if stage.Kind != "tar-app" {
		return nil
	}
	dataDir := ""
	for i := 0; i < len(stage.LaunchArgs); i++ {
		if stage.LaunchArgs[i] == "-data" && i+1 < len(stage.LaunchArgs) {
			dataDir = stage.LaunchArgs[i+1]
			i++
		}
	}
	if dataDir == "" {
		return fmt.Errorf("chưa xác định thư mục dữ liệu để cập nhật an toàn")
	}
	app, err := resolveExistingParents(stage.Target)
	if err != nil {
		return err
	}
	data, err := resolveExistingParents(dataDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(app, data)
	if err != nil {
		return err
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("dữ liệu đang nằm trong Biz Studio.app; hãy chuyển thư mục dữ liệu ra ngoài ứng dụng và mở lại với -data trỏ đến thư mục mới trước khi cập nhật")
	}
	return nil
}

func resolveExistingParents(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) || filepath.Dir(abs) == abs {
		return "", err
	}
	parent, err := resolveExistingParents(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}
