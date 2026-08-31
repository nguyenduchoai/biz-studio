package setup

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const windowsProbeFramePrefix = "BIZSTUDIO_WINDOWS_READINESS_V1:"

type windowsProbe struct {
	Protocol        int    `json:"protocol"`
	FirewallReady   bool   `json:"firewallReady"`
	NetworkReady    bool   `json:"networkReady"`
	NetworkCategory string `json:"networkCategory"`
}

// parseWindowsProbeOutput chỉ đọc payload có marker riêng. Windows PowerShell
// có thể chen warning, information, CLIXML hoặc BOM vào output dù exit code 0;
// Base64 giữ phần JSON ở ASCII và không phụ thuộc code page của máy.
func parseWindowsProbeOutput(out []byte) (windowsProbe, error) {
	normalized := string(bytes.ReplaceAll(out, []byte{0}, nil))
	var frames []string
	for _, line := range strings.Split(strings.ReplaceAll(normalized, "\r\n", "\n"), "\n") {
		if i := strings.Index(line, windowsProbeFramePrefix); i >= 0 {
			frames = append(frames, strings.TrimSpace(line[i+len(windowsProbeFramePrefix):]))
		}
	}
	if len(frames) == 0 {
		return windowsProbe{}, fmt.Errorf("thiếu payload kiểm tra Windows")
	}
	if len(frames) != 1 {
		return windowsProbe{}, fmt.Errorf("nhận %d payload kiểm tra Windows", len(frames))
	}
	raw, err := base64.StdEncoding.DecodeString(frames[0])
	if err != nil {
		return windowsProbe{}, fmt.Errorf("payload Windows không phải Base64 hợp lệ: %w", err)
	}
	var probe windowsProbe
	if err := json.Unmarshal(raw, &probe); err != nil {
		return windowsProbe{}, fmt.Errorf("payload Windows không phải JSON hợp lệ: %w", err)
	}
	if probe.Protocol != 1 {
		return windowsProbe{}, fmt.Errorf("protocol Windows không hỗ trợ: %d", probe.Protocol)
	}
	return probe, nil
}
