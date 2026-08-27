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

func TestStreamingForcesPythonUTF8ForWindowsTranscript(t *testing.T) {
	env := strings.Join(pythonUTF8Env([]string{"PATH=test"}), "\n")
	for _, want := range []string{"PYTHONUTF8=1", "PYTHONIOENCODING=utf-8"} {
		if !strings.Contains(env, want) {
			t.Errorf("runStreaming thiếu %s", want)
		}
	}
}
