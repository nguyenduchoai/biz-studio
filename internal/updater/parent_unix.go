//go:build !windows

package updater

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func retryableRenameError(err error) bool { return errors.Is(err, os.ErrPermission) }

func waitForParent(pid int, timeout time.Duration) error {
	if pid <= 0 {
		// Backward compatibility for update specs written by older versions.
		time.Sleep(2 * time.Second)
		return nil
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("kiểm tra tiến trình trước khi cập nhật: %w", err)
		}
		if time.Now().After(deadline) {
			return errors.New("ứng dụng chưa thoát; chưa thay file cập nhật")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
