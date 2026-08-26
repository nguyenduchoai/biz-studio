//go:build !windows

package store

import "os"

func secureFilePermissions(path string) error {
	return os.Chmod(path, 0o600)
}
