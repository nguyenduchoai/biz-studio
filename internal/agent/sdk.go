package agent

import (
	"bytes"
	"io"
	"os/exec"
	"path/filepath"

	"bizstudio/internal/agentsdk"
	"bizstudio/internal/store"
)

func (r *Runner) buildSDKCmd(projectID, prompt, resumeID string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
	return r.buildSDKCmdWithSettings(projectID, prompt, resumeID, r.st.Settings())
}

func (r *Runner) buildSDKCmdWithSettings(projectID, prompt, resumeID string, cfg store.Settings) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
	cmd, err := agentsdk.Command(r.dataDir, filepath.Join(r.dataDir, "projects", projectID),
		prompt, resumeID, cfg.AnthropicAPIKey, cfg.ClaudeSDKBudgetUSD)
	if err != nil {
		return nil, nil, nil, err
	}
	var stderr bytes.Buffer
	// Vendor diagnostics can contain request headers. Only sanitized protocol
	// errors on stdout reach the session log; never persist raw SDK stderr.
	cmd.Stderr = io.Discard
	out, err := cmd.StdoutPipe()
	return cmd, out, &stderr, err
}
