package setup

import (
	"strings"
	"testing"
)

func TestSDKInstallersForceUTF8OnEveryPythonInvocation(t *testing.T) {
	for _, name := range []string{"setup-claude-sdk.ps1", "setup-claude-sdk.sh"} {
		body, err := scriptFile(name)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		for _, line := range strings.Split(string(body), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "& $Python ") && !strings.HasPrefix(line, "& $VenvPy ") && !strings.HasPrefix(line, "$ARCH_PREFIX ") {
				continue
			}
			calls++
			if !strings.Contains(line, " -X utf8 ") {
				t.Errorf("%s must force UTF-8 for Python (isolated mode ignores PYTHONUTF8): %s", name, line)
			}
		}
		if calls != 4 {
			t.Errorf("%s: expected four verified Python invocations, got %d", name, calls)
		}
	}
}
