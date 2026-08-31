package setup

import (
	"encoding/base64"
	"strings"
	"testing"
)

func framedWindowsProbe(t *testing.T, json string) string {
	t.Helper()
	return windowsProbeFramePrefix + base64.StdEncoding.EncodeToString([]byte(json))
}

func TestParseWindowsProbeOutputIgnoresPowerShellNoise(t *testing.T) {
	out := strings.Join([]string{
		"\ufeffWARNING: module emitted a localized warning",
		"#< CLIXML diagnostic from Windows PowerShell",
		framedWindowsProbe(t, `{"protocol":1,"firewallReady":true,"networkReady":true,"networkCategory":"Private"}`),
		"VERBOSE: trailing host output",
	}, "\r\n")

	probe, err := parseWindowsProbeOutput([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if probe.Protocol != 1 || !probe.FirewallReady || !probe.NetworkReady || probe.NetworkCategory != "Private" {
		t.Fatalf("probe = %+v", probe)
	}
}

func TestParseWindowsProbeOutputRejectsMissingBrokenOrDuplicateFrame(t *testing.T) {
	valid := framedWindowsProbe(t, `{"protocol":1,"firewallReady":false,"networkReady":false,"networkCategory":"Public"}`)
	tests := map[string]string{
		"missing":   "WARNING: no machine-readable result",
		"base64":    windowsProbeFramePrefix + "%%%",
		"json":      framedWindowsProbe(t, `{not-json}`),
		"protocol":  framedWindowsProbe(t, `{"protocol":2}`),
		"duplicate": valid + "\r\n" + valid,
	}
	for name, out := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWindowsProbeOutput([]byte(out)); err == nil {
				t.Fatal("muốn lỗi nhưng parser trả thành công")
			}
		})
	}
}
