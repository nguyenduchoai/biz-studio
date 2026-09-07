package setup

import (
	"strings"
	"testing"
)

func TestInstallerOutputKeepsWindowsCRLFLines(t *testing.T) {
	var lines []string
	pump(strings.NewReader("Preparing Python\r\nInstalling package\r\nDone\r\n"), func(line string) {
		lines = append(lines, line)
	})
	if got := strings.Join(lines, "|"); got != "Preparing Python|Installing package|Done" {
		t.Fatalf("Windows installer log disappeared: %q", got)
	}
}

func TestSafeInstallerEnvRemovesCredentialsButKeepsRuntimePaths(t *testing.T) {
	got := safeInstallerEnv([]string{
		"PATH=C:\\Windows", "TEMP=C:\\Temp", "HTTPS_PROXY=http://proxy.local",
		"ANTHROPIC_API_KEY=secret", "GITHUB_TOKEN=secret", "AWS_SECRET_ACCESS_KEY=secret",
		"GITHUB_PAT=secret", "SESSION_JWT=secret", "RANDOM_CONFIG=secret",
	})
	want := map[string]bool{
		"PATH=C:\\Windows": true, "TEMP=C:\\Temp": true, "HTTPS_PROXY=http://proxy.local": true,
	}
	if len(got) != len(want) {
		t.Fatalf("env sau lọc = %v", got)
	}
	for _, item := range got {
		if !want[item] {
			t.Fatalf("env không mong muốn còn lại: %s", item)
		}
	}
}

func TestInstallerOutputRedactsCredentials(t *testing.T) {
	got := redactInstallerLine("proxy https://user:pass@example.com token=abc123 sk-ant-secretvalue")
	for _, secret := range []string{"user:pass", "abc123", "secretvalue"} {
		if strings.Contains(got, secret) {
			t.Fatalf("output còn lộ %q: %s", secret, got)
		}
	}
}
