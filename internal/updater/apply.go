package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Stage struct {
	Archive    string   `json:"archive"`
	Tag        string   `json:"tag"`
	Kind       string   `json:"kind"`
	Target     string   `json:"target"`
	LaunchPath string   `json:"launchPath"`
	LaunchArgs []string `json:"launchArgs"`
	ParentPID  int      `json:"parentPid,omitempty"`
}

func NewStage(archive, tag, goos string) Stage {
	exe, _ := os.Executable()
	exe, _ = filepath.Abs(exe)
	s := Stage{Archive: archive, Tag: safeTag(tag), Target: filepath.Dir(exe), LaunchPath: exe}
	switch goos {
	case "windows":
		s.Kind = "zip-dir"
	case "linux":
		s.Kind = "tar-dir"
	case "darwin":
		if app := appBundle(exe); app != "" {
			s.Kind = "tar-app"
			s.Target = app
			s.LaunchPath = app
		}
	}
	return s
}

func Start(stage Stage) error {
	if stage.Archive == "" || stage.Kind == "" || stage.Target == "" {
		return errors.New("thông tin cài cập nhật chưa đầy đủ")
	}
	if err := validateDataOutsideBundle(stage); err != nil {
		return err
	}
	// Check write access before the HTTP handler shuts down the working app.
	parent := stage.Target
	if stage.Kind == "tar-app" {
		parent = filepath.Dir(stage.Target)
	}
	probe, err := os.MkdirTemp(parent, ".bizstudio-update-check-")
	if err != nil {
		return fmt.Errorf("không ghi được thư mục ứng dụng; hãy chuyển ứng dụng vào thư mục có quyền ghi trước khi cập nhật: %w", err)
	}
	_ = os.Remove(probe)
	stage.ParentPID = os.Getpid()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(stage.Archive)
	ext := filepath.Ext(exe)
	helper := filepath.Join(dir, "bizstudio-update-helper"+ext)
	if err := copyFile(exe, helper, 0o700); err != nil {
		return fmt.Errorf("chuẩn bị bộ cập nhật: %w", err)
	}
	spec := filepath.Join(dir, "apply.json")
	raw, err := json.Marshal(stage)
	if err != nil {
		return err
	}
	if err := os.WriteFile(spec, raw, 0o600); err != nil {
		return err
	}
	cmd := exec.Command(helper, "__apply-update", spec)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("khởi động bộ cập nhật: %w", err)
	}
	return cmd.Process.Release()
}

func Apply(specPath string) error {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	var stage Stage
	if err := json.Unmarshal(raw, &stage); err != nil {
		return err
	}
	if err := waitForParent(stage.ParentPID, 30*time.Second); err != nil {
		return err
	}
	tx, err := prepareUpdate(stage)
	if err != nil {
		// Extraction/replacement failed with the original version still present.
		stage.Tag = ""
		return errors.Join(err, relaunch(stage))
	}
	return finishUpdate(tx, stage, relaunch)
}

func applyStage(stage Stage) error {
	tx, err := prepareUpdate(stage)
	if err != nil {
		return err
	}
	return tx.commit()
}

func prepareUpdate(stage Stage) (*updateTransaction, error) {
	parent := stage.Target
	if stage.Kind == "tar-app" {
		parent = filepath.Dir(stage.Target)
	}
	tmp, err := os.MkdirTemp(parent, ".bizstudio-update-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	switch stage.Kind {
	case "zip-dir":
		if err := extractZip(stage.Archive, tmp); err != nil {
			return nil, err
		}
		return replaceFilesTransaction(tmp, stage.Target)
	case "tar-dir":
		if err := extractTarGz(stage.Archive, tmp); err != nil {
			return nil, err
		}
		return replaceFilesTransaction(tmp, stage.Target)
	case "tar-app":
		if err := extractTarGz(stage.Archive, tmp); err != nil {
			return nil, err
		}
		return replaceAppTransaction(filepath.Join(tmp, "Biz Studio.app"), stage.Target)
	default:
		return nil, fmt.Errorf("kiểu cập nhật không hỗ trợ: %s", stage.Kind)
	}
}

func replaceFiles(src, dst string) error {
	tx, err := replaceFilesTransaction(src, dst)
	if err != nil {
		return err
	}
	return tx.commit()
}

func relaunch(stage Stage) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" && stage.Kind == "tar-app" {
		// Launch the binary directly: older bundle launchers discard --args,
		// resetting the user's data directory and port after an update.
		cmd = exec.Command(filepath.Join(stage.LaunchPath, "Contents", "MacOS", "bizstudio"), stage.LaunchArgs...)
	} else {
		cmd = exec.Command(stage.LaunchPath, stage.LaunchArgs...)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return awaitRelaunch(cmd, stage, 30*time.Second)
}

func appBundle(exe string) string {
	clean := filepath.Clean(exe)
	marker := ".app" + string(filepath.Separator)
	i := strings.Index(clean, marker)
	if i < 0 {
		return ""
	}
	return clean[:i+len(".app")]
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func safeArchivePath(root, name string) (string, error) {
	name = filepath.Clean(filepath.FromSlash(name))
	if name == "." {
		return root, nil
	}
	if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("gói cập nhật có đường dẫn không an toàn: %s", name)
	}
	return filepath.Join(root, name), nil
}

func extractZip(path, dst string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, item := range zr.File {
		target, err := safeArchivePath(dst, item.Name)
		if err != nil {
			return err
		}
		if item.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		r, err := item.Open()
		if err != nil {
			return err
		}
		if err := writeReader(target, r, item.Mode().Perm()); err != nil {
			r.Close()
			return err
		}
		r.Close()
	}
	return nil
}

func extractTarGz(path, dst string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeArchivePath(dst, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := writeReader(target, tr, os.FileMode(h.Mode).Perm()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("gói cập nhật chứa loại file không hỗ trợ: %s", h.Name)
		}
	}
}

func writeReader(path string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
