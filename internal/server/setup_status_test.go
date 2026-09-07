package server

import (
	"context"
	"testing"
)

func TestPythonCheckDoesNotMarkCancelledProbeReady(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if status := checkPython(ctx); status.OK {
		t.Fatalf("cancelled Python probe marked ready: %+v", status)
	}
}
