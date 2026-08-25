package agent

import (
	"slices"
	"strings"
	"testing"

	"bizstudio/internal/store"
)

func TestClaudeRunnerUsesRestrictedNonInteractivePermissions(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := New(st, func(string, any) {}, st.DataDir)
	cmd, stdout, _, err := runner.buildCmd("project-1", "render", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = stdout.Close()
	joined := strings.Join(cmd.Args, " ")
	if strings.Contains(joined, "dangerously-skip-permissions") {
		t.Fatalf("runner còn bypass permission: %s", joined)
	}
	for _, want := range []string{"--safe-mode", "--permission-mode", "dontAsk", "--allowedTools"} {
		if !slices.Contains(cmd.Args, want) {
			t.Errorf("runner thiếu %q: %v", want, cmd.Args)
		}
	}
}

func TestSafeAgentEnvDoesNotPassCloudCredentials(t *testing.T) {
	got := safeAgentEnv([]string{"PATH=/bin", "HOME=/home/a", "ANTHROPIC_API_KEY=secret", "AWS_PROFILE=prod"})
	if strings.Join(got, "|") != "PATH=/bin|HOME=/home/a" {
		t.Fatalf("safeAgentEnv = %v", got)
	}
}
