package publishpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"bizstudio/internal/gemini"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// metaLLM — cấu trúc JSON mà LLM phải trả về.
type metaLLM struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Hashtags    []string `json:"hashtags"`
}

// generateMeta sinh metadata xuất bản: thử Gemini → Claude CLI → meta cơ bản.
// Không trả lỗi vì luôn có fallback; các lỗi trung gian được ghi log.
func generateMeta(ctx context.Context, st *store.Store, p *store.Project) map[string]any {
	system, user := metaPrompt(p)

	if txt, err := geminiText(ctx, st, system, user); err != nil {
		st.AddLog("warn", "publish", "Gemini không khả dụng, thử Claude CLI: "+err.Error())
	} else if m, perr := parseMetaJSON(txt); perr != nil {
		st.AddLog("warn", "publish", "Gemini trả JSON không hợp lệ, thử Claude CLI: "+perr.Error())
	} else {
		return metaMap(m, "gemini")
	}

	if txt, err := claudeText(ctx, st, system+"\n\n"+user); err != nil {
		st.AddLog("warn", "publish", "Claude CLI không khả dụng, dùng meta cơ bản: "+err.Error())
	} else if m, perr := parseMetaJSON(txt); perr != nil {
		st.AddLog("warn", "publish", "Claude trả JSON không hợp lệ, dùng meta cơ bản: "+perr.Error())
	} else {
		return metaMap(m, "claude")
	}

	return metaMap(basicMeta(p), "basic")
}

// metaPrompt dựng prompt (system, user) yêu cầu LLM trả JSON metadata.
func metaPrompt(p *store.Project) (system, user string) {
	var s strings.Builder
	s.WriteString("Bạn là chuyên gia đặt tiêu đề và mô tả video mạng xã hội.\n")
	s.WriteString("Dựa trên thông tin dự án video được cung cấp, hãy trả về DUY NHẤT một JSON object ")
	s.WriteString(`dạng {"title":"...","description":"...","hashtags":["#..."]} — không kèm giải thích, không markdown.` + "\n")
	s.WriteString("- title: tiêu đề hấp dẫn, tiếng Việt, dưới 100 ký tự.\n")
	s.WriteString("- description: mô tả 2-4 câu, tiếng Việt.\n")
	s.WriteString("- hashtags: 5-8 hashtag bắt đầu bằng #, không dấu cách.")

	var u strings.Builder
	u.WriteString("Tên dự án: " + p.Name + "\n")
	if p.BriefDesc != "" {
		u.WriteString("Mô tả gốc: " + p.BriefDesc + "\n")
	}
	if len(p.Keywords) > 0 {
		u.WriteString("Từ khoá: " + strings.Join(p.Keywords, ", ") + "\n")
	}
	return s.String(), u.String()
}

// geminiText gọi Gemini qua client dùng chung (Settings), timeout 60s.
func geminiText(ctx context.Context, st *store.Store, system, user string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return gemini.NewFromSettings(st).GenerateText(cctx, system, user)
}

// claudeText gọi claude CLI: `claude -p --output-format text`, prompt qua stdin, timeout 120s.
func claudeText(ctx context.Context, st *store.Store, prompt string) (string, error) {
	cfg := st.Settings()
	bin := cfg.ClaudeBin
	if bin == "" {
		bin = "claude"
	}
	if !util.Exists(bin) {
		return "", fmt.Errorf("chưa cài claude CLI (%s)", bin)
	}
	cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	args := []string{"-p", "--output-format", "text"}
	if cfg.ClaudeModel != "" {
		args = append(args, "--model", cfg.ClaudeModel)
	}
	cmd := exec.CommandContext(cctx, bin, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude CLI lỗi: %v — %s", err, truncate(se.String(), 300))
	}
	if strings.TrimSpace(so.String()) == "" {
		return "", fmt.Errorf("claude CLI trả nội dung rỗng")
	}
	return so.String(), nil
}

// parseMetaJSON tách JSON object từ text LLM (bỏ fence ```json nếu có).
func parseMetaJSON(s string) (metaLLM, error) {
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return metaLLM{}, fmt.Errorf("không tìm thấy JSON object trong phản hồi")
	}
	var m metaLLM
	if err := json.Unmarshal([]byte(s[i:j+1]), &m); err != nil {
		return metaLLM{}, fmt.Errorf("parse JSON thất bại: %w", err)
	}
	if strings.TrimSpace(m.Title) == "" {
		return metaLLM{}, fmt.Errorf("JSON thiếu trường title")
	}
	return m, nil
}

// basicMeta — fallback khi không có LLM nào khả dụng.
func basicMeta(p *store.Project) metaLLM {
	desc := p.BriefDesc
	if desc == "" {
		desc = "Video được dựng bằng Biz Studio."
	}
	var tags []string
	for _, k := range p.Keywords {
		k = strings.TrimSpace(strings.TrimPrefix(k, "#"))
		if k == "" {
			continue
		}
		tags = append(tags, "#"+strings.ReplaceAll(k, " ", ""))
	}
	if len(tags) == 0 {
		tags = []string{"#video", "#bizstudio"}
	}
	return metaLLM{Title: p.Name, Description: desc, Hashtags: tags}
}

func metaMap(m metaLLM, source string) map[string]any {
	if m.Hashtags == nil {
		m.Hashtags = []string{}
	}
	return map[string]any{
		"title":       strings.TrimSpace(m.Title),
		"description": strings.TrimSpace(m.Description),
		"hashtags":    m.Hashtags,
		"metaSource":  source,
	}
}

// addProbeInfo thêm width/height/fps/duration của video vào meta (nếu ffprobe có).
func addProbeInfo(ctx context.Context, st *store.Store, meta map[string]any, videoPath string) {
	if !util.Exists("ffprobe") {
		st.AddLog("warn", "publish", "Không tìm thấy ffprobe — meta.json thiếu thông số video")
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := util.Run(cctx, "ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,avg_frame_rate",
		"-show_entries", "format=duration",
		"-of", "json", videoPath)
	if err != nil {
		st.AddLog("warn", "publish", "ffprobe lỗi: "+err.Error())
		return
	}
	var pr struct {
		Streams []struct {
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			AvgFrameRate string `json:"avg_frame_rate"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		st.AddLog("warn", "publish", "ffprobe trả JSON không đọc được: "+err.Error())
		return
	}
	if len(pr.Streams) > 0 {
		meta["width"] = pr.Streams[0].Width
		meta["height"] = pr.Streams[0].Height
		if fps := parseFPS(pr.Streams[0].AvgFrameRate); fps > 0 {
			meta["fps"] = fps
		}
	}
	if d, err := strconv.ParseFloat(pr.Format.Duration, 64); err == nil {
		meta["duration"] = d
	}
}

// parseFPS chuyển "30000/1001" hoặc "30/1" thành số thực (làm tròn 2 chữ số).
func parseFPS(s string) float64 {
	parts := strings.SplitN(s, "/", 2)
	num, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil || num <= 0 {
		return 0
	}
	den := 1.0
	if len(parts) == 2 {
		den, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil || den <= 0 {
			return 0
		}
	}
	return float64(int(num/den*100+0.5)) / 100
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
