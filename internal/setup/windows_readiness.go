package setup

import "context"

const WindowsFirewallRuleName = "Biz Studio - QR Upload"

// WindowsReadiness là phần kiểm tra nhẹ chạy khi mở ứng dụng. Firewall chỉ
// áp dụng cho đúng binary Biz Studio và profile Private/Domain; control API
// vẫn chỉ bind loopback nên rule này không làm lộ trang quản trị ra LAN.
type WindowsReadiness struct {
	Supported        bool   `json:"supported"`
	WinGetReady      bool   `json:"winGetReady"`
	FirewallReady    bool   `json:"firewallReady"`
	NetworkReady     bool   `json:"networkReady"`
	PhoneReady       bool   `json:"phoneReady"`
	NetworkCategory  string `json:"networkCategory,omitempty"`
	NeedsPreparation bool   `json:"needsPreparation"`
	RuleName         string `json:"ruleName,omitempty"`
	Detail           string `json:"detail,omitempty"`
}

// CheckWindowsReadiness không thay đổi hệ thống; implementation Windows nằm
// ở file build-tag riêng để các bản macOS/Linux vẫn dùng cùng API.
func CheckWindowsReadiness(ctx context.Context) WindowsReadiness {
	return finalizeWindowsReadiness(checkWindowsReadiness(ctx))
}

func finalizeWindowsReadiness(status WindowsReadiness) WindowsReadiness {
	status.PhoneReady = status.Supported && status.FirewallReady && status.NetworkReady
	// Network Public cần người dùng tự đổi sang Private. Đưa trạng thái này vào
	// wizard để có cảnh báo rõ ràng, nhưng Biz Studio tuyệt đối không tự đổi
	// chính sách mạng của công ty/gia đình.
	status.NeedsPreparation = status.Supported && (!status.WinGetReady || !status.FirewallReady || !status.NetworkReady)
	return status
}

// ConfigureWindowsFirewall xin UAC rồi chỉ tạo rule QR cho executable hiện tại.
func ConfigureWindowsFirewall(ctx context.Context) error {
	return configureWindowsFirewall(ctx)
}
