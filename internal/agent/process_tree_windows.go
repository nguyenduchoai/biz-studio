//go:build windows

package agent

import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

type agentProcessTree struct {
	terminate func()
	close     func()
}

func prepareAgentProcess(cmd *exec.Cmd) {
	// No agent code may run before job ownership is established: a child
	// spawned between Start and AssignProcessToJobObject would escape the job.
	cmd.SysProcAttr = &windows.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_SUSPENDED}
}

func attachAgentProcess(process *os.Process) (*agentProcessTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("CreateJobObject: %w", err)
	}
	cleanup := func() { _ = windows.CloseHandle(job) }
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		cleanup()
		return nil, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(process.Pid))
	if err != nil {
		cleanup()
		return nil, err
	}
	err = windows.AssignProcessToJobObject(job, handle)
	_ = windows.CloseHandle(handle)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := resumeAgentPrimaryThread(uint32(process.Pid)); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		cleanup()
		return nil, fmt.Errorf("ResumeThread: %w", err)
	}
	return &agentProcessTree{terminate: func() { _ = windows.TerminateJobObject(job, 1) }, close: cleanup}, nil
}

// Go closes CreateProcess's primary-thread handle. Recover it with documented
// Toolhelp APIs while CREATE_SUSPENDED guarantees application code has not run.
// Ambiguous/injected threads fail closed rather than resume an arbitrary one.
func resumeAgentPrimaryThread(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	var threadID uint32
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		if threadID != 0 {
			return fmt.Errorf("không xác định được duy nhất luồng khởi động")
		}
		threadID = entry.ThreadID
	}
	if err != windows.ERROR_NO_MORE_FILES {
		return err
	}
	if threadID == 0 {
		return fmt.Errorf("không tìm thấy luồng khởi động")
	}
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, threadID)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(thread)
	previous, err := windows.ResumeThread(thread)
	if err != nil {
		return err
	}
	if previous != 1 {
		return fmt.Errorf("trạng thái tạm dừng không hợp lệ: %d", previous)
	}
	return nil
}
