package setup

import "testing"

func TestFinalizeWindowsReadinessRequiresPrivateNetwork(t *testing.T) {
	status := finalizeWindowsReadiness(WindowsReadiness{
		Supported:     true,
		WinGetReady:   true,
		FirewallReady: true,
		NetworkReady:  false,
	})
	if !status.NeedsPreparation || status.PhoneReady {
		t.Fatalf("mạng Public phải mở lại wizard và chặn QR ready: %+v", status)
	}
}

func TestFinalizeWindowsReadinessReady(t *testing.T) {
	status := finalizeWindowsReadiness(WindowsReadiness{
		Supported:     true,
		WinGetReady:   true,
		FirewallReady: true,
		NetworkReady:  true,
	})
	if status.NeedsPreparation || !status.PhoneReady {
		t.Fatalf("máy đủ điều kiện lại chưa ready: %+v", status)
	}
}
