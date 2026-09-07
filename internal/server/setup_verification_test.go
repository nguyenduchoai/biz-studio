package server

import (
	"strings"
	"testing"
)

func TestSetupVerificationFailureKeepsActionableCause(t *testing.T) {
	detail := "FFmpeg thiếu bộ lọc subtitles — cần cài bộ FFmpeg đầy đủ"
	if got := setupVerificationFailure(detail); !strings.Contains(got, detail) || strings.Contains(got, "mở lại") {
		t.Fatalf("must report real failed capability, not a misleading restart instruction: %s", got)
	}
	if got := setupVerificationFailure(" "); !strings.Contains(got, "kiểm tra lại") {
		t.Fatalf("missing fallback: %s", got)
	}
}
