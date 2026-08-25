//go:build windows

package setup

import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processTree struct {
	terminate func()
	close     func()
}

func prepareProcessTree(_ *exec.Cmd) {}

// attachProcessTree đặt PowerShell/WinGet và toàn bộ tiến trình con vào Job
// Object. Khi hủy cài hoặc backend dừng, đóng handle sẽ diệt cả cây thay vì để
// pip/msiexec mồ côi tiếp tục ghi dở ở nền.
func attachProcessTree(process *os.Process) (*processTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("CreateJobObject: %w", err)
	}
	cleanup := func() { _ = windows.CloseHandle(job) }

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		cleanup()
		return nil, fmt.Errorf("SetInformationJobObject: %w", err)
	}

	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false, uint32(process.Pid))
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("OpenProcess: %w", err)
	}
	err = windows.AssignProcessToJobObject(job, handle)
	_ = windows.CloseHandle(handle)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	return &processTree{
		terminate: func() { _ = windows.TerminateJobObject(job, 1) },
		close:     cleanup,
	}, nil
}
