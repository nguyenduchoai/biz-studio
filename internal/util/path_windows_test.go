//go:build windows

package util

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestRegistryPATHExpandsWindowsVariables(t *testing.T) {
	base := filepath.Join(t.TempDir(), "Công cụ máy tính")
	t.Setenv("BIZSTUDIO_TEST_BIN", base)
	got := splitWindowsPATH(` "%BIZSTUDIO_TEST_BIN%\Scripts" ; ; C:\Windows\System32`)
	want := []string{filepath.Join(base, "Scripts"), `C:\Windows\System32`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registry PATH = %#v, want %#v", got, want)
	}
}
