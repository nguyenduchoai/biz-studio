package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type instanceLock struct {
	file *os.File
}

func acquireInstanceLock(dataDir string) (*instanceLock, bool, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(filepath.Join(dataDir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	locked, err := tryLockInstanceFile(f)
	if err != nil {
		_ = f.Close()
		return nil, false, fmt.Errorf("khóa dữ liệu: %w", err)
	}
	if !locked {
		_ = f.Close()
		return nil, false, nil
	}
	return &instanceLock{file: f}, true, nil
}

func (l *instanceLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlockInstanceFile(l.file)
	closeErr := l.file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
