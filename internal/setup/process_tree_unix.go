//go:build !windows

package setup

import (
	"os"
	"os/exec"
	"syscall"
)

type processTree struct {
	terminate func()
	close     func()
}

func prepareProcessTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachProcessTree(process *os.Process) (*processTree, error) {
	return &processTree{
		terminate: func() { _ = syscall.Kill(-process.Pid, syscall.SIGKILL) },
		close:     func() {},
	}, nil
}
