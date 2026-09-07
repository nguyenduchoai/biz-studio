package updater

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func retryableRenameError(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}

func waitForParent(pid int, timeout time.Duration) error {
	if pid <= 0 {
		time.Sleep(2 * time.Second)
		return nil
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil // The original application has already exited.
	}
	if err != nil {
		return fmt.Errorf("kiểm tra tiến trình trước khi cập nhật: %w", err)
	}
	defer windows.CloseHandle(handle)
	state, err := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	if err != nil {
		return err
	}
	if state != windows.WAIT_OBJECT_0 {
		return errors.New("ứng dụng chưa thoát; chưa thay file cập nhật")
	}
	return nil
}
