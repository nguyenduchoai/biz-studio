// Package gemini — REST client gọi Google Gemini API (generateContent):
// sinh văn bản, phân tích file đa phương tiện, tạo ảnh và giọng đọc (TTS).
package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/store"
)

const (
	defaultBase  = "https://generativelanguage.googleapis.com"
	defaultModel = "gemini-2.5-flash"
	maxInlineMB  = 18
)

// ErrNoKey — chưa có API key trong trang Cấu hình & API.
var ErrNoKey = errors.New("chưa cấu hình Gemini API key (Cấu hình & API)")

// Client — REST client Gemini.
type Client struct {
	Key   string
	Base  string
	Model string
	// Model riêng cho ảnh và giọng đọc — model văn bản không làm được hai việc này.
	ImageModel string
	TTSModel   string
	HTTP       *http.Client
}

// NewFromSettings tạo client từ Settings trong store (key/base/model).
func NewFromSettings(st *store.Store) *Client {
	cfg := st.Settings()
	base := strings.TrimRight(strings.TrimSpace(cfg.GeminiBase), "/")
	if base == "" {
		base = defaultBase
	}
	model := strings.TrimSpace(cfg.GeminiModel)
	if model == "" {
		model = defaultModel
	}
	return &Client{
		Key:        strings.TrimSpace(cfg.GeminiAPIKey),
		Base:       base,
		Model:      model,
		ImageModel: strings.TrimSpace(cfg.GeminiImageModel),
		TTSModel:   strings.TrimSpace(cfg.GeminiTTSModel),
		HTTP:       &http.Client{Timeout: 180 * time.Second},
	}
}

// ---------- request / response ----------

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type reqPart struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type reqContent struct {
	Parts []reqPart `json:"parts"`
}

type genRequest struct {
	Contents          []reqContent `json:"contents"`
	SystemInstruction *reqContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  any          `json:"generationConfig,omitempty"`
}

type respInline struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type respPart struct {
	Text       string      `json:"text"`
	InlineData *respInline `json:"inlineData"`
}

type genResponse struct {
	Candidates []struct {
		Content struct {
			Parts []respPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// generate gọi POST {Base}/v1beta/models/{model}:generateContent?key={Key}.
func (c *Client) generate(ctx context.Context, model string, req genRequest) (*genResponse, error) {
	if c.Key == "" {
		return nil, ErrNoKey
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mã hoá request Gemini: %w", err)
	}
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.Base, model, c.Key)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("tạo request Gemini: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gọi Gemini API thất bại: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("đọc response Gemini: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API lỗi (HTTP %d): %s", resp.StatusCode, apiErrMessage(data))
	}
	var out genResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("giải mã response Gemini: %w", err)
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("Gemini không trả về nội dung nào (candidates rỗng)")
	}
	return &out, nil
}

// apiErrMessage lấy error.message từ body JSON lỗi của Gemini.
func apiErrMessage(data []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

func joinTextParts(resp *genResponse) string {
	var b strings.Builder
	for _, p := range resp.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	return strings.TrimSpace(b.String())
}

// GenerateText sinh văn bản từ prompt (system tùy chọn).
func (c *Client) GenerateText(ctx context.Context, system, user string) (string, error) {
	req := genRequest{Contents: []reqContent{{Parts: []reqPart{{Text: user}}}}}
	if strings.TrimSpace(system) != "" {
		req.SystemInstruction = &reqContent{Parts: []reqPart{{Text: system}}}
	}
	resp, err := c.generate(ctx, c.Model, req)
	if err != nil {
		return "", err
	}
	text := joinTextParts(resp)
	if text == "" {
		return "", errors.New("Gemini không trả về văn bản nào")
	}
	return text, nil
}

var mimeByExt = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".mp3": "audio/mp3", ".wav": "audio/wav", ".mp4": "video/mp4",
	".m4a": "audio/m4a", ".aac": "audio/aac", ".flac": "audio/flac",
}

// GenerateWithFiles gửi prompt kèm các file media (inline base64, đoán mime theo đuôi file).
func (c *Client) GenerateWithFiles(ctx context.Context, prompt string, paths []string) (string, error) {
	parts := []reqPart{{Text: prompt}}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return "", fmt.Errorf("không đọc được file %s: %w", p, err)
		}
		if info.Size() > maxInlineMB*1024*1024 {
			return "", fmt.Errorf("file %s quá lớn (%.1fMB, giới hạn %dMB) — hãy dùng file nhỏ hơn (cắt ngắn hoặc nén lại trước)",
				filepath.Base(p), float64(info.Size())/(1024*1024), maxInlineMB)
		}
		mime, ok := mimeByExt[strings.ToLower(filepath.Ext(p))]
		if !ok {
			return "", fmt.Errorf("định dạng file %s chưa hỗ trợ gửi lên Gemini (hỗ trợ: jpg, png, mp3, wav, mp4, m4a, aac, flac)", filepath.Base(p))
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("không đọc được file %s: %w", p, err)
		}
		parts = append(parts, reqPart{InlineData: &inlineData{MimeType: mime, Data: base64.StdEncoding.EncodeToString(raw)}})
	}
	resp, err := c.generate(ctx, c.Model, genRequest{Contents: []reqContent{{Parts: parts}}})
	if err != nil {
		return "", err
	}
	text := joinTextParts(resp)
	if text == "" {
		return "", errors.New("Gemini không trả về văn bản nào")
	}
	return text, nil
}
