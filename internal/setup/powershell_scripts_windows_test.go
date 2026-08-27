//go:build windows

package setup

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestEmbeddedPowerShellSetupScriptsParse(t *testing.T) {
	ps, err := systemPowerShell()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"setup-whisper.ps1", "setup-vieneu.ps1"} {
		t.Run(name, func(t *testing.T) {
			body, err := scriptFile(name)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command",
				`$source = [Console]::In.ReadToEnd(); [void][scriptblock]::Create($source)`)
			cmd.Stdin = bytes.NewReader(body)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("PowerShell parse lỗi: %v\n%s", err, out)
			}
		})
	}
}
