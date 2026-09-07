//go:build !windows

package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A fake interpreter exercises the complete shell flow without downloading a
// model or touching system Python. No pip executable is created: scripts must
// use the selected venv interpreter, including when its path contains spaces.
const setupPythonFixture = `#!/bin/sh
case "$1" in
  -I) exit 0 ;;
  -m)
    if [ "$2" = "venv" ]; then
      mkdir -p "$3/bin"
      cp "$0" "$3/bin/python"
      exit 0
    fi
    case "$*" in
      *--upgrade*) exit "${FIXTURE_UPGRADE_EXIT:-0}" ;;
      *faster-whisper*) exit "${FIXTURE_PACKAGE_EXIT:-0}" ;;
      *) exit 0 ;;
    esac ;;
  -c) exit "${FIXTURE_IMPORT_EXIT:-0}" ;;
  -) cat >/dev/null; exit "${FIXTURE_MODEL_EXIT:-0}" ;;
esac
exit 42
`

func TestWhisperShellUsesSelectedPythonAndSeparatesOptionalDownloads(t *testing.T) {
	for _, tc := range []struct {
		name, variable string
		wantSuccess    bool
	}{
		{"selected interpreter with spaces", "", true},
		{"pip upgrade offline", "FIXTURE_UPGRADE_EXIT", true},
		{"model prefetch offline", "FIXTURE_MODEL_EXIT", true},
		{"package install failed", "FIXTURE_PACKAGE_EXIT", false},
		{"native import failed", "FIXTURE_IMPORT_EXIT", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			py := filepath.Join(root, "selected python")
			if err := os.WriteFile(py, []byte(setupPythonFixture), 0o700); err != nil {
				t.Fatal(err)
			}
			// Force a non-Apple host in this isolated shell fixture. Architecture
			// selection itself is checked by PythonRuntime/native Windows tests.
			if err := os.WriteFile(filepath.Join(root, "uname"), []byte("#!/bin/sh\nprintf 'Linux\\n'\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "python3"), []byte("#!/bin/sh\nexit 41\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			body, err := scriptFile("setup-whisper.sh")
			if err != nil {
				t.Fatal(err)
			}
			script := filepath.Join(root, "setup.sh")
			if err := os.WriteFile(script, body, 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("/bin/bash", script, filepath.Join(root, "data with spaces"))
			cmd.Env = append(os.Environ(), "PATH="+root+":/usr/bin:/bin", "BIZSTUDIO_PYTHON="+py, "SKIP_MODEL=0")
			if tc.variable != "" {
				cmd.Env = append(cmd.Env, tc.variable+"=1")
			}
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.wantSuccess {
				t.Fatalf("exit=%v, want success=%v\n%s", err, tc.wantSuccess, out)
			}
			if strings.Contains(string(out), "✅ Xong!") != tc.wantSuccess {
				t.Fatalf("misleading final status:\n%s", out)
			}
		})
	}
}
