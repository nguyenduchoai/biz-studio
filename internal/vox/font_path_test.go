package vox

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDrawTitleUsesFontWithSpecialPath(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	var font []byte
	candidates := append([]string{}, fontCandidates...)
	candidates = append(candidates, filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf"), "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil {
			font = data
			break
		}
	}
	if font == nil {
		t.Skip("no system font for fixture")
	}
	path := filepath.Join(t.TempDir(), "phông O'Brien,[1].ttf")
	if err := os.WriteFile(path, font, 0o600); err != nil {
		t.Fatal(err)
	}
	previous := fontCandidates
	fontCandidates = []string{path}
	t.Cleanup(func() { fontCandidates = previous })
	filter := drawTitleFilter("Test", 320)
	frame, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=black:size=320x240:duration=0.2", "-vf", filter, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "gray", "pipe:1").Output()
	if err != nil {
		t.Fatalf("fontfile filter failed: %v", err)
	}
	bright := 0
	for _, pixel := range frame {
		if pixel > 100 {
			bright++
		}
	}
	if bright < 30 {
		t.Fatalf("title was not drawn using the font fixture: %d bright pixels", bright)
	}
}
