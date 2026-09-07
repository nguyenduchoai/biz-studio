package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"time"
)

const windowGenerationFile = ".appwindow-generation"

type windowLaunch struct {
	openedAt   time.Time
	dataDir    string
	generation string
}

// A second EXE cannot share memory with the original window watcher. Publish a
// small generation marker before opening another window, so that watcher will
// not later shut down the server below the newly opened window when work ends.
func recordWindowLaunch(dataDir string) windowLaunch {
	launch := windowLaunch{openedAt: time.Now(), dataDir: dataDir}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		log.Printf("Không tạo được dấu mở cửa sổ: %v", err)
		return launch
	}
	launch.generation = hex.EncodeToString(token[:])
	if err := os.WriteFile(filepath.Join(dataDir, windowGenerationFile), []byte(launch.generation), 0o600); err != nil {
		log.Printf("Không ghi được dấu mở cửa sổ: %v", err)
		launch.generation = ""
	}
	return launch
}

func (launch windowLaunch) superseded() bool {
	if launch.generation == "" {
		return true
	}
	current, err := os.ReadFile(filepath.Join(launch.dataDir, windowGenerationFile))
	return err != nil || string(current) != launch.generation
}
