//go:build !windows

package agentsdk

import (
	"golang.org/x/sys/unix"
	"os"
)

func lockRuntimeFile(f *os.File, exclusive bool) error {
	mode := unix.LOCK_SH
	if exclusive {
		mode = unix.LOCK_EX
	}
	return unix.Flock(int(f.Fd()), mode|unix.LOCK_NB)
}

func unlockRuntimeFile(f *os.File) { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }
