package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"bizstudio/internal/gemini"
	"bizstudio/internal/htmlvideo"
	"bizstudio/internal/openaiapi"
)

func (s *Server) routesHTMLVideo(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tools/htmlvideo/plan", s.handleHTMLVideoPlan)
	mux.HandleFunc("POST /api/tools/htmlvideo", s.handleHTMLVideoRender)
}

// handleHTMLVideoPlan — LLM đồng bộ (≤180s) sinh kịch bản []htmlvideo.Scene
// từ prompt / content / README GitHub. Ưu tiên engine: openai → gemini → claude CLI.
func (s *Server) handleHTMLVideoPlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt        string `json:"prompt"`
		Content       string `json:"content"`
		RepoURL       string `json:"repoUrl"`
		Count         int    `json:"count"`
		Style         string `json:"style"`
		TemplateGuide string `json:"templateGuide"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}

	var sources []string
	if c := strings.TrimSpace(body.Content); c != "" {
		sources = append(sources, "Nội dung:\n"+c)
	}
	if repo := strings.TrimSpace(body.RepoURL); repo != "" {
		readme, err := fetchGithubReadme(repo)
		if err != nil {
			httpErr(w, http.StatusBadRequest, "đọc README từ GitHub thất bại: %v", err)
			return
		}
		sources = append(sources, fmt.Sprintf("README của dự án %s:\n%s", repo, readme))
	}
	if p := strings.TrimSpace(body.Prompt); p != "" {
		sources = append(sources, "Yêu cầu thêm từ người dùng:\n"+p)
	}
	// Khuôn đặt SAU yêu cầu của người dùng và nói rõ là hướng dẫn dựng, không
	// phải nội dung: nếu để lẫn, model dễ đem chính câu chữ trong khuôn ("Mở đầu
	// bằng một câu gây sốc") vào lời thoại của cảnh.
	if g := strings.TrimSpace(body.TemplateGuide); g != "" {
		sources = append(sources, "Khuôn dựng cần theo (đây là HƯỚNG DẪN cách kể, "+
			"tuyệt đối không đưa nguyên văn mấy dòng này vào lời thoại):\n"+g)
	}
	if len(sources) == 0 {
		httpErr(w, http.StatusBadRequest, "cần ít nhất một nguồn nội dung: prompt, content hoặc repoUrl")
		return
	}

	count := body.Count
	if count <= 0 {
		count = 8
	}
	style := strings.TrimSpace(body.Style)
	if style == "" {
		style = "hiện đại, cuốn hút"
	}
	prompt := buildHTMLVideoPlanPrompt(count, style, strings.Join(sources, "\n\n"))

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	raw, err := s.planLLMText(ctx, prompt)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "tạo kịch bản thất bại: %v", err)
		return
	}

	var scenes []htmlvideo.Scene
	if err := json.Unmarshal([]byte(extractJSONArray(stripCodeFence(raw))), &scenes); err != nil {
		httpErr(w, http.StatusInternalServerError, "LLM trả về JSON không hợp lệ: %v", err)
		return
	}
	if len(scenes) == 0 {
		httpErr(w, http.StatusInternalServerError, "LLM không trả về cảnh nào")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scenes": scenes})
}

// buildHTMLVideoPlanPrompt — prompt mô tả schema Scene + quy tắc chọn template.
func buildHTMLVideoPlanPrompt(count int, style, source string) string {
	return fmt.Sprintf(`Bạn là đạo diễn video ngắn dạng "video as code". Tạo kịch bản video khoảng %d cảnh, phong cách %s, từ nguồn nội dung bên dưới.

Trả về DUY NHẤT một JSON mảng các cảnh theo đúng schema:
[{"template":"hero|bullets|code|chart|product|quote|outro","title":"tiêu đề ngắn","subtitle":"phụ đề (tuỳ chọn)","bullets":["ý 1","ý 2"],"code":"lệnh hoặc đoạn code","image":"từ khóa tiếng Anh tìm ảnh minh họa","chart":[{"label":"nhãn","value":12.5}],"voiceText":"lời đọc tiếng Việt","duration":4,"accent":"#RRGGBB (tuỳ chọn)"}]

Quy tắc:
- Cảnh ĐẦU TIÊN phải là "hero" (mở đầu giới thiệu), cảnh CUỐI CÙNG phải là "outro" (kêu gọi hành động).
- Có số liệu / so sánh → dùng "chart" (2-6 mục, field chart). Có lệnh cài đặt hoặc đoạn code → dùng "code" (field code).
- Liệt kê tính năng / ý chính → "bullets" (3-5 ý ngắn gọn). Trích dẫn / nhận định → "quote" (title = câu trích, subtitle = tác giả). Giới thiệu bằng hình ảnh → "product" (field image = từ khóa tiếng Anh).
- Mỗi cảnh có "voiceText" là 1-2 câu tiếng Việt tự nhiên, đúng nội dung cảnh đó.
- "duration" mỗi cảnh 3-7 giây. Chỉ điền các field cần cho template của cảnh.
- Không giải thích, không markdown, không văn bản nào khác ngoài JSON.

Nguồn nội dung:
%s`, count, style, source)
}

// planLLMText gọi LLM theo thứ tự ưu tiên: API Trực Tiếp (OpenAI-compatible)
// → Gemini → Claude CLI.
func (s *Server) planLLMText(ctx context.Context, prompt string) (string, error) {
	const system = "Bạn là trợ lý tạo kịch bản video. Luôn trả về đúng JSON được yêu cầu, không thêm bất kỳ nội dung nào khác."
	set := s.st.Settings()
	if strings.TrimSpace(set.OpenAIKey) != "" {
		return openaiapi.NewFromSettings(s.st).ChatText(ctx, system, prompt)
	}
	if strings.TrimSpace(set.GeminiAPIKey) != "" {
		return gemini.NewFromSettings(s.st).GenerateText(ctx, system, prompt)
	}
	return runClaudePrompt(ctx, set.ClaudeBin, system+"\n\n"+prompt)
}

var githubRepoRe = regexp.MustCompile(`github\.com[/:]([\w.-]+)/([\w.-]+)`)

const readmeMaxChars = 12000

// fetchGithubReadme tải README.md raw của repo GitHub (nhánh main, fallback
// master), cắt tối đa 12000 ký tự.
func fetchGithubReadme(repoURL string) (string, error) {
	m := githubRepoRe.FindStringSubmatch(repoURL)
	if m == nil {
		return "", fmt.Errorf("repoUrl không hợp lệ: %q (cần dạng github.com/owner/repo)", repoURL)
	}
	owner, repo := m[1], strings.TrimSuffix(m[2], ".git")
	client := &http.Client{Timeout: 20 * time.Second}
	var lastErr error
	for _, branch := range []string{"main", "master"} {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/README.md", owner, repo, branch)
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d với nhánh %s", resp.StatusCode, branch)
			continue
		}
		text := string(data)
		if runes := []rune(text); len(runes) > readmeMaxChars {
			text = string(runes[:readmeMaxChars])
		}
		if strings.TrimSpace(text) == "" {
			lastErr = fmt.Errorf("README nhánh %s rỗng", branch)
			continue
		}
		return text, nil
	}
	return "", fmt.Errorf("không tải được README của %s/%s (main/master): %v", owner, repo, lastErr)
}

// handleHTMLVideoRender — job kind=htmlvideo render video từ các cảnh HTML.
// Có projectId → render vào ProjectDir/outputs + cập nhật project; không →
// data/tmp/htmlvideo-<id>.
func (s *Server) handleHTMLVideoRender(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID string            `json:"projectId"`
		Scenes    []htmlvideo.Scene `json:"scenes"`
		Config    htmlvideo.Config  `json:"config"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if len(body.Scenes) == 0 {
		httpErr(w, http.StatusBadRequest, "không có cảnh nào để render")
		return
	}

	pid := strings.TrimSpace(body.ProjectID)
	var workDir string
	if pid != "" {
		if _, ok := s.st.Project(pid); !ok {
			httpErr(w, http.StatusNotFound, "không tìm thấy dự án: %s", pid)
			return
		}
		workDir = filepath.Join(s.ProjectDir(pid), "outputs")
	} else {
		workDir = filepath.Join(s.DataDir, "tmp", s.st.NewID("htmlvideo"))
	}

	j := s.Jobs.Submit("htmlvideo", pid, "Render HTML Video: "+shortText(sceneJobLabel(body.Scenes), 50), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
		defer cancel()
		mp4, err := htmlvideo.Render(ctx, s.st, body.Scenes, body.Config, workDir, upd)
		if err != nil {
			return "", err
		}
		out := s.toolRelPath(mp4)
		if pid != "" {
			if p, ok := s.st.Project(pid); ok {
				p.OutputFile = out
				p.Status = "done"
				p.Progress = 6
				s.st.SaveProject(&p)
				s.Hub.Broadcast("project", p)
			}
		}
		return out, nil
	})
	writeJSON(w, http.StatusOK, j)
}

// sceneJobLabel — nhãn hiển thị job theo cảnh đầu tiên có tiêu đề.
func sceneJobLabel(scenes []htmlvideo.Scene) string {
	for _, sc := range scenes {
		if t := strings.TrimSpace(sc.Title); t != "" {
			return t
		}
	}
	return fmt.Sprintf("%d cảnh", len(scenes))
}
