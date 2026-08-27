//go:build windows

package setup

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestConfigureFirewallScriptIsScopedToProgramAndPrivateNetworks(t *testing.T) {
	path := `C:\Program Files\Biz Studio\Biz Studio.exe`
	script := configureFirewallScript(path)
	for _, want := range []string{
		WindowsFirewallRuleName,
		`$PSHOME 'Modules\NetSecurity\NetSecurity.psd1'`,
		`NetSecurity\New-NetFirewallRule`,
		`-Program $exePath`,
		`-Protocol TCP`,
		`-Profile Private,Domain`,
		`-Direction Inbound`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script thiếu %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "Public") || strings.Contains(script, "LocalPort 6868") {
		t.Fatalf("script mở profile/cổng điều khiển ngoài phạm vi:\n%s", script)
	}
}

func TestProbeWindowsScriptMatchesCurrentExecutable(t *testing.T) {
	script := probeWindowsScript(`D:\Portable\Biz Studio.exe`, `192.168.1.25`)
	for _, want := range []string{
		`$PSHOME 'Modules\NetSecurity\NetSecurity.psd1'`,
		`$PSHOME 'Modules\NetConnection\NetConnection.psd1'`,
		`$PSHOME 'Modules\NetTCPIP\NetTCPIP.psd1'`,
		"NetSecurity\\Get-NetFirewallApplicationFilter",
		"NetSecurity\\Get-NetFirewallPortFilter",
		"NetSecurity\\Get-NetFirewallAddressFilter",
		"NetConnection\\Get-NetConnectionProfile",
		"NetTCPIP\\Get-NetIPAddress",
		"192.168.1.25",
		"$_.Program",
		"$_.Protocol",
		"$_.LocalPort",
		"$_.RemotePort",
		"$rules.Count -eq 1",
		"$profileMask -eq 3",
		"$hasPublicNetwork",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("probe thiếu %q:\n%s", want, script)
		}
	}
}

func TestPowerShellEncodingUsesUTF16LE(t *testing.T) {
	want := "Xin chào Windows"
	raw, err := base64.StdEncoding.DecodeString(encodePowerShell(want))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("UTF-16LE có số byte lẻ: %d", len(raw))
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = uint16(raw[i*2]) | uint16(raw[i*2+1])<<8
	}
	if got := string(utf16.Decode(units)); got != want {
		t.Fatalf("decode = %q, muốn %q", got, want)
	}
}

func TestPowerShellSingleQuoteEscaping(t *testing.T) {
	if got := psQuote(`C:\Users\O'Brien\Biz Studio.exe`); got != `C:\Users\O''Brien\Biz Studio.exe` {
		t.Fatalf("psQuote = %q", got)
	}
}
