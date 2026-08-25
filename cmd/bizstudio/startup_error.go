package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func configureStartupLogging(dataDir string) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dataDir, "bizstudio-startup.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
	}
}

func fatalStartup(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	log.Printf("KHÔNG KHỞI ĐỘNG ĐƯỢC: %s", message)
	showStartupError(message)
	os.Exit(1)
}
