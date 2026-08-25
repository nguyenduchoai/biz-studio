//go:build windows

package main

import "golang.org/x/sys/windows"

func showStartupError(message string) {
	text, _ := windows.UTF16PtrFromString(message + "\n\nChi tiết đã ghi trong bizstudio-startup.log ở thư mục dữ liệu.")
	title, _ := windows.UTF16PtrFromString("Biz Studio không khởi động được")
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}
