package whisper

import (
	"strings"
	"testing"
)

func TestRunnerUsesWindowsFriendlyHubDownloadSettings(t *testing.T) {
	for _, want := range []string{
		`HF_HUB_DISABLE_XET`,
		`HF_HUB_DISABLE_SYMLINKS_WARNING`,
		`HF_HUB_DOWNLOAD_TIMEOUT`,
	} {
		if !strings.Contains(whisperRunner, want) {
			t.Errorf("whisper runner thiếu %s", want)
		}
	}
}
