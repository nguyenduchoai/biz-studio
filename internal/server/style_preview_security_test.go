package server

import (
	"strings"
	"testing"
)

func TestAppendPreviewSeekRunsInsideSandboxedDocument(t *testing.T) {
	got := appendPreviewSeek("<html><body><div id=stage></div></body></html>", 2.5)
	if !strings.Contains(got, "window.seek(2.5)") {
		t.Fatalf("preview không tự gọi seek: %s", got)
	}
	if strings.Index(got, "window.seek(2.5)") > strings.Index(got, "</body>") {
		t.Fatal("preview seek bị chèn sau </body>")
	}
}
