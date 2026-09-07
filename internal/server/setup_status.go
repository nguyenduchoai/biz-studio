package server

import (
	"context"
	"io"
	"os/exec"
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
	filters, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-filters").CombinedOutput()
	if err != nil {
		return toolCheck{Detail: "không kiểm tra được bộ lọc FFmpeg — hãy kiểm tra lại hoặc cài bộ FFmpeg đầy đủ"}
	}
	if missing := missingFFmpegFilters(string(filters)); len(missing) > 0 {
		return toolCheck{Detail: "FFmpeg thiếu bộ lọc " + strings.Join(missing, ", ") + " — cần cài bộ FFmpeg đầy đủ để dựng phụ đề và chữ"}
	}
	return toolCheck{OK: true, Detail: ffmpeg.Detail + " · ffprobe, phụ đề và chữ OK"}
}

func missingFFmpegFilters(output string) []string {
	found := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.Contains(fields[2], "->") {
			found[fields[1]] = true
		}
	}
	var missing []string
	for _, required := range []string{"subtitles", "ass", "drawtext"} {
		if !found[required] {
			missing = append(missing, required)
		}
	}
	return missing
}

func checkPython(ctx context.Context) toolCheck {
	util.AugmentPATH()
	py, err := setup.FindPython(ctx)
	if err != nil {
		return toolCheck{Detail: err.Error()}
	}
	return toolCheck{OK: true, Detail: py.Version}
}

func setupVerificationFailure(detail string) string {
	if strings.TrimSpace(detail) == "" {
		detail = "Chưa nhận diện được công cụ; mở lại Biz Studio rồi kiểm tra lại."
	}
	return "Bộ cài đã chạy xong nhưng công cụ chưa sẵn sàng. " + detail
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
