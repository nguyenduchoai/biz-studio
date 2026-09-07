package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type replacedPath struct {
	target string
	backup string
}

type updateTransaction struct {
	dir   string
	paths []replacedPath
}

func (tx *updateTransaction) commit() error { return os.RemoveAll(tx.dir) }

func (tx *updateTransaction) rollback() error {
	var errs []error
	for i := len(tx.paths) - 1; i >= 0; i-- {
		p := tx.paths[i]
		if err := os.RemoveAll(p.target); err != nil {
			errs = append(errs, err)
			continue
		}
		if p.backup != "" {
			if err := renameWithRetry(p.backup, p.target); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("khôi phục chưa hoàn tất; bản sao còn ở %s: %w", tx.dir, errors.Join(errs...))
	}
	return tx.commit()
}

func finishUpdate(tx *updateTransaction, stage Stage, launch func(Stage) error) error {
	if err := launch(stage); err != nil {
		if restoreErr := tx.rollback(); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		stage.Tag = "" // The restored version is expected to be older.
		return errors.Join(fmt.Errorf("bản mới không khởi động được; đã khôi phục bản trước: %w", err), launch(stage))
	}
	return tx.commit()
}

func replaceFilesTransaction(src, dst string) (*updateTransaction, error) {
	dir, err := os.MkdirTemp(dst, ".bizstudio-backup-")
	if err != nil {
		return nil, err
	}
	tx := &updateTransaction{dir: dir}
	err = filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return tx.replace(path, target, info.Mode().Perm())
	})
	if err != nil {
		return nil, errors.Join(err, tx.rollback())
	}
	return tx, nil
}

func (tx *updateTransaction) replace(src, target string, mode os.FileMode) error {
	info, statErr := os.Lstat(target)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if statErr == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("không thể cập nhật file đang là thư mục hoặc liên kết: %s", target)
	}
	// Backup and replacement live on the target filesystem for atomic renames.
	next := filepath.Join(tx.dir, fmt.Sprintf("%d.new", len(tx.paths)))
	if err := copyFile(src, next, mode); err != nil {
		return err
	}
	p := replacedPath{target: target}
	if statErr == nil {
		p.backup = filepath.Join(tx.dir, fmt.Sprintf("%d.old", len(tx.paths)))
		if err := renameWithRetry(target, p.backup); err != nil {
			return err
		}
	}
	// Record before installing so a failed rename restores this file too.
	tx.paths = append(tx.paths, p)
	return renameWithRetry(next, target)
}

func replaceAppTransaction(src, dst string) (*updateTransaction, error) {
	for _, rel := range []string{"Contents/MacOS/bizstudio", "Contents/MacOS/launcher", "Contents/Info.plist"} {
		info, err := os.Stat(filepath.Join(src, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("gói cập nhật macOS thiếu %s hợp lệ", rel)
		}
	}
	dir, err := os.MkdirTemp(filepath.Dir(dst), ".bizstudio-backup-")
	if err != nil {
		return nil, err
	}
	tx := &updateTransaction{dir: dir}
	backup := filepath.Join(dir, "Biz Studio.app")
	if err := os.Rename(dst, backup); err != nil {
		_ = tx.commit()
		return nil, fmt.Errorf("không thể thay ứng dụng hiện tại: %w", err)
	}
	tx.paths = append(tx.paths, replacedPath{target: dst, backup: backup})
	if err := os.Rename(src, dst); err != nil {
		return nil, errors.Join(err, tx.rollback())
	}
	return tx, nil
}

func renameWithRetry(src, dst string) error {
	var err error
	for i := 0; i < 40; i++ {
		if err = os.Rename(src, dst); err == nil {
			return nil
		}
		if !retryableRenameError(err) {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("không thay được %s: %w", filepath.Base(dst), err)
}
