package agent

import (
	"bytes"
	"io"
	"os"
	"os/exec"
)

// Own pipe lifetimes instead of Cmd.StdoutPipe: Cmd.Wait closes that reader
// before a concurrent parser can drain it. Direct file descriptors let Wait
// observe the leader exiting, kill inherited-pipe descendants, then drain EOF.
type agentPipes struct {
	stdout, stderr       *os.File
	outWriter, errWriter *os.File
	errTarget            io.Writer
}

func ownAgentPipes(cmd *exec.Cmd, previous io.ReadCloser, stderr *bytes.Buffer) (*agentPipes, error) {
	p := &agentPipes{errTarget: stderr}
	if cmd.Stderr == io.Discard {
		p.errTarget = io.Discard
	}
	var err error
	p.stdout, p.outWriter, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	p.stderr, p.errWriter, err = os.Pipe()
	if err != nil {
		p.close()
		return nil, err
	}
	if previous != nil {
		_ = previous.Close()
	}
	cmd.Stdout, cmd.Stderr = p.outWriter, p.errWriter
	return p, nil
}

func (p *agentPipes) closeWriters() {
	if p.outWriter != nil {
		_ = p.outWriter.Close()
	}
	if p.errWriter != nil {
		_ = p.errWriter.Close()
	}
}

func (p *agentPipes) closeReaders() {
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
	}
}

func (p *agentPipes) close() { p.closeWriters(); p.closeReaders() }
