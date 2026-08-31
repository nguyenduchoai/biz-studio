//go:build windows

package setup

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"bizstudio/internal/util"
)

func checkWindowsReadiness(ctx context.Context) WindowsReadiness {
	status := WindowsReadiness{Supported: true, RuleName: WindowsFirewallRuleName}
	status.WinGetReady = wingetAvailable(ctx)
	exe, err := os.Executable()
	if err != nil {
		status.Detail = "không xác định được vị trí Biz Studio: " + err.Error()
		return status
	}
	exe, _ = filepath.Abs(exe)
	ps, err := systemPowerShell()
	if err != nil {
		status.Detail = err.Error()
		return status
	}
	out, err := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePowerShell(probeWindowsScript(exe, util.LanIP()))).CombinedOutput()
	if err != nil {
		status.Detail = "không kiểm tra được Windows Firewall: " + compactPowerShellError(out, err)
		return status
	}
	probe, err := parseWindowsProbeOutput(out)
	if err != nil {
		status.Detail = "Windows Firewall trả kết quả không hợp lệ: " + err.Error()
		return status
	}
	status.FirewallReady = probe.FirewallReady
	status.NetworkReady = probe.NetworkReady
	status.NetworkCategory = probe.NetworkCategory
	return status
}

func configureWindowsFirewall(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("không xác định được vị trí Biz Studio: %w", err)
	}
	exe, _ = filepath.Abs(exe)
	ps, err := systemPowerShell()
	if err != nil {
		return err
	}
	inner := encodePowerShell(configureFirewallScript(exe))
	outer := `[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$ErrorActionPreference = 'Stop'
$argsForAdmin = @('-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-EncodedCommand','` + inner + `')
$process = Start-Process -FilePath '` + psQuote(ps) + `' -Verb RunAs -Wait -PassThru -ArgumentList $argsForAdmin
exit $process.ExitCode`
	out, err := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePowerShell(outer)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("không cấu hình được Firewall; hãy chọn Yes ở cửa sổ UAC: %s", compactPowerShellError(out, err))
	}
	verified := checkWindowsReadiness(ctx)
	if !verified.FirewallReady {
		if verified.Detail != "" {
			return fmt.Errorf("đã chạy quyền quản trị nhưng chưa xác minh được Firewall: %s", verified.Detail)
		}
		return fmt.Errorf("đã chạy quyền quản trị nhưng rule %q chưa áp dụng cho Biz Studio", WindowsFirewallRuleName)
	}
	return nil
}

func systemPowerShell() (string, error) {
	root := strings.TrimSpace(os.Getenv("SystemRoot"))
	if root == "" {
		root = strings.TrimSpace(os.Getenv("WINDIR"))
	}
	if root == "" {
		return "", fmt.Errorf("Windows không cung cấp thư mục SystemRoot")
	}
	ps := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(ps); err != nil {
		return "", fmt.Errorf("không tìm thấy Windows PowerShell hệ thống: %w", err)
	}
	return ps, nil
}

func wingetAvailable(ctx context.Context) bool {
	path, err := exec.LookPath("winget")
	if err != nil {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(probeCtx, path, "--version").CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func probeWindowsScript(exe, lanIP string) string {
	return `[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$ProgressPreference = 'SilentlyContinue'
$netSecurityModule = Join-Path $PSHOME 'Modules\NetSecurity\NetSecurity.psd1'
$netConnectionModule = Join-Path $PSHOME 'Modules\NetConnection\NetConnection.psd1'
$netTCPIPModule = Join-Path $PSHOME 'Modules\NetTCPIP\NetTCPIP.psd1'
Import-Module -Name $netSecurityModule -Force -ErrorAction Stop | Out-Null
Import-Module -Name $netConnectionModule -Force -ErrorAction Stop | Out-Null
Import-Module -Name $netTCPIPModule -Force -ErrorAction Stop | Out-Null
$exePath = '` + psQuote(exe) + `'
$lanIP = '` + psQuote(lanIP) + `'
$ruleName = '` + psQuote(WindowsFirewallRuleName) + `'
$firewallReady = $false
$rules = @(NetSecurity\Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue)
if ($rules.Count -eq 1) {
  $rule = $rules[0]
  $profileMask = [int]$rule.Profile
  $ruleReady = $rule.Enabled -eq 'True' -and $rule.Direction -eq 'Inbound' -and
    $rule.Action -eq 'Allow' -and $profileMask -eq 3
  $appReady = [bool](NetSecurity\Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $rule -ErrorAction SilentlyContinue |
    Where-Object { $_.Program -ieq $exePath })
  $tcpReady = [bool](NetSecurity\Get-NetFirewallPortFilter -AssociatedNetFirewallRule $rule -ErrorAction SilentlyContinue |
    Where-Object {
      (([string]$_.Protocol) -ieq 'TCP' -or $_.Protocol -eq 6) -and
      ([string]$_.LocalPort) -eq 'Any' -and ([string]$_.RemotePort) -eq 'Any'
    })
  $addressReady = [bool](NetSecurity\Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $rule -ErrorAction SilentlyContinue |
    Where-Object { ([string]$_.LocalAddress) -eq 'Any' -and ([string]$_.RemoteAddress) -eq 'Any' })
  $firewallReady = $ruleReady -and $appReady -and $tcpReady -and $addressReady
}
$profiles = @()
$ipInfo = @(NetTCPIP\Get-NetIPAddress -IPAddress $lanIP -AddressFamily IPv4 -ErrorAction SilentlyContinue)
if ($ipInfo.Count -gt 0) {
  $profiles = @(NetConnection\Get-NetConnectionProfile -InterfaceIndex $ipInfo[0].InterfaceIndex -ErrorAction SilentlyContinue)
}
$categories = @($profiles | ForEach-Object { [string]$_.NetworkCategory } | Sort-Object -Unique)
$hasPrivateNetwork = [bool]($categories | Where-Object { $_ -eq 'Private' -or $_ -eq 'DomainAuthenticated' })
$hasPublicNetwork = [bool]($categories | Where-Object { $_ -eq 'Public' })
$networkReady = $hasPrivateNetwork -and -not $hasPublicNetwork
$payload = [pscustomobject]@{
  protocol = 1
  firewallReady = [bool]$firewallReady
  networkReady = [bool]$networkReady
  networkCategory = ($categories -join ', ')
} | ConvertTo-Json -Compress
$payloadBytes = [Text.Encoding]::UTF8.GetBytes($payload)
[Console]::Out.WriteLine('` + windowsProbeFramePrefix + `' + [Convert]::ToBase64String($payloadBytes))`
}

func configureFirewallScript(exe string) string {
	return `[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$ErrorActionPreference = 'Stop'
$netSecurityModule = Join-Path $PSHOME 'Modules\NetSecurity\NetSecurity.psd1'
Import-Module -Name $netSecurityModule -Force -ErrorAction Stop
$exePath = '` + psQuote(exe) + `'
$ruleName = '` + psQuote(WindowsFirewallRuleName) + `'
NetSecurity\Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue | NetSecurity\Remove-NetFirewallRule
NetSecurity\New-NetFirewallRule -DisplayName $ruleName -Description 'Cho phép điện thoại gửi video, ảnh và âm thanh vào Biz Studio trong mạng nội bộ.' -Direction Inbound -Action Allow -Program $exePath -Protocol TCP -Profile Private,Domain -Enabled True | Out-Null`
}

func encodePowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	raw := make([]byte, len(units)*2)
	for i, unit := range units {
		raw[i*2] = byte(unit)
		raw[i*2+1] = byte(unit >> 8)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func psQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func compactPowerShellError(out []byte, err error) string {
	message := strings.TrimSpace(string(out))
	if message == "" {
		return err.Error()
	}
	if len(message) > 500 {
		message = message[:500] + "…"
	}
	return message
}
