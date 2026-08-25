//go:build !windows

package setup

import (
	"context"
	"testing"
	"time"
)

func TestCancelStopsInstallerProcessGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	plan := &Plan{Steps: []Step{{Bin: "sh", Args: []string{"-c", "sleep 60 & wait"}}}}
	done := make(chan error, 1)
	go func() { done <- Run(ctx, plan, func(string) {}) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel trả nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel không dừng cây tiến trình installer")
	}
}
