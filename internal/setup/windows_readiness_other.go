//go:build !windows

package setup

import (
	"context"
	"errors"
)

func checkWindowsReadiness(context.Context) WindowsReadiness {
	return WindowsReadiness{}
}

func configureWindowsFirewall(context.Context) error {
	return errors.New("thiết lập Firewall chỉ hỗ trợ trên Windows")
}
