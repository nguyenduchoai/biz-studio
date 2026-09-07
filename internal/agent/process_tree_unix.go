//go:build !windows

package agent

import (
	"os"
	"os/exec"
	"syscall"
)

type agentProcessTree struct {
	terminate func()
	close     func()
}

func prepareAgentProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachAgentProcess(process *os.Process) (*agentProcessTree, error) {
	return &agentProcessTree{
		terminate: func() { _ = syscall.Kill(-process.Pid, syscall.SIGKILL) },
		close:     func() {},
	}, nil
}
