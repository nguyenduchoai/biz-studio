package server

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"bizstudio/internal/setup"
	"bizstudio/internal/util"
)

func (s *Server) setupStatuses(ctx context.Context) []toolStatus {
	tools := setup.Tools()
	out := make([]toolStatus, len(tools))
	var wg sync.WaitGroup
	for i, tool := range tools {
		wg.Add(1)
		go func(i int, tool setup.Tool) {
			defer wg.Done()
			out[i] = s.setupToolStatus(ctx, tool)
		}(i, tool)
	}
	wg.Wait()
	return out
}

func (s *Server) setupToolStatus(ctx context.Context, tool setup.Tool) toolStatus {
	cfg := s.st.Settings()
	status := toolStatus{Tool: tool, Running: setupIsRunning(tool.ID) || setupIsRunning(fullSetupID)}
	var check toolCheck
	switch tool.ID {
	case "git":
		check = checkBinVersion(ctx, "git", "--version")
	case "python":
		check = checkPython(ctx)
	case "ffmpeg":
		check = checkFFmpeg(ctx)
	case "ytdlp":
		check = checkYtdlp(ctx, binOrDefault(cfg.YtdlpBin, "yt-dlp"))
	case "chrome":
		check = checkChrome(ctx, cfg)
	case "claude":
		return s.checkClaudeSetup(ctx, tool, cfg.ClaudeBin)
	case "vieneu":
		check = s.checkVieNeu(ctx)
	case "whisper":
		check = s.checkWhisper(ctx)
	default:
		check = toolCheck{Detail: "chưa có cách kiểm tra"}
	}
	status.Installed = check.OK
	status.Ready = check.OK
	status.Detail = check.Detail
	return status
}

func checkFFmpeg(ctx context.Context) toolCheck {
	ffmpeg := checkBinVersion(ctx, "ffmpeg", "-version")
	if !ffmpeg.OK {
		return ffmpeg
	}
	ffprobe := checkBinVersion(ctx, "ffprobe", "-version")
	if !ffprobe.OK {
		return toolCheck{Detail: "đã có ffmpeg nhưng thiếu ffprobe — cần cài lại bộ FFmpeg đầy đủ"}
	}
	return toolCheck{OK: true, Detail: ffmpeg.Detail + " · ffprobe OK"}
}

func checkPython(ctx context.Context) toolCheck {
	type candidate struct {
		bin  string
		args []string
	}
	candidates := []candidate{{"python3", []string{"--version"}}, {"python", []string{"--version"}}}
	if runtime.GOOS == "windows" {
		candidates = append([]candidate{{"py", []string{"-3", "--version"}}}, candidates...)
	}
	for _, c := range candidates {
		if !util.Exists(c.bin) {
			continue
		}
		res := checkBinVersion(ctx, c.bin, c.args...)
		if res.OK && pythonVersionSupported(res.Detail) {
			return res
		}
	}
	return toolCheck{Detail: "cần Python 3.10 trở lên (bộ cài Full dùng Python 3.11)"}
}

func pythonVersionSupported(version string) bool {
	var major, minor int
	_, err := fmt.Sscanf(strings.TrimSpace(version), "Python %d.%d", &major, &minor)
	return err == nil && (major > 3 || major == 3 && minor >= 10)
}

func (s *Server) checkClaudeSetup(ctx context.Context, tool setup.Tool, configured string) toolStatus {
	status := toolStatus{Tool: tool, Running: setupIsRunning(tool.ID) || setupIsRunning(fullSetupID)}
	bin := binOrDefault(configured, "claude")
	installed, ready, detail := claudeReadyState(ctx, bin)
	status.Installed = installed
	status.Ready = ready
	status.NeedsLogin = installed && !ready
	status.Detail = detail
	return status
}

func checkClaudeReady(ctx context.Context, bin string) toolCheck {
	_, ready, detail := claudeReadyState(ctx, bin)
	return toolCheck{OK: ready, Detail: detail}
}

func claudeReadyState(ctx context.Context, bin string) (bool, bool, string) {
	version := checkBinVersion(ctx, bin, "--version")
	if !version.OK {
		return false, false, version.Detail
	}
	ready := claudeLoggedIn(ctx, bin)
	if ready {
		return true, true, version.Detail + " · đã đăng nhập"
	}
	return true, false, version.Detail + " · đã cài, cần đăng nhập Claude"
}

func claudeLoggedIn(ctx context.Context, bin string) bool {
	cmd := exec.CommandContext(ctx, bin, "auth", "status")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}
