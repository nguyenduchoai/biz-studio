package util

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// Run chạy lệnh, trả stdout. Lỗi kèm stderr để dễ debug.
func Run(ctx context.Context, name string, args ...string) (string, error) {
	out, errOut, err := RunErr(ctx, name, args...)
	if err != nil {
		return out, fmt.Errorf("%s: %w — %s", name, err, truncate(errOut, 500))
	}
	return out, nil
}

// RunErr chạy lệnh, trả cả stdout lẫn stderr.
func RunErr(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	return so.String(), se.String(), err
}

// Exists kiểm tra binary có trong PATH không.
func Exists(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// LanIP trả IP LAN của máy (cho QR kết nối điện thoại).
func LanIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
