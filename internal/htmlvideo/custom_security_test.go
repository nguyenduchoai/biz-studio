package htmlvideo

import (
	"strings"
	"testing"

	"bizstudio/internal/store"
)

func TestCustomHTMLAlwaysReceivesRestrictiveCSP(t *testing.T) {
	for _, body := range []string{
		`<div id="stage">ok</div>`,
		`<!doctype html><html><head></head><body><div id="stage"></div></body></html>`,
	} {
		got := buildCustomHTML(&store.StyleKit{CustomHTML: body}, Scene{}, "")
		for _, want := range []string{"Content-Security-Policy", "connect-src 'none'", "object-src 'none'"} {
			if !strings.Contains(got, want) {
				t.Fatalf("custom HTML thiếu %q: %s", want, got)
			}
		}
	}
}
