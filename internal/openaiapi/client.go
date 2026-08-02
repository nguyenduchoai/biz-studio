// Package openaiapi — client "API Trực Tiếp": gọi endpoint chat/completions
// tương thích OpenAI (OpenAI, LM Studio, Ollama, OpenRouter…).
package openaiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bizstudio/internal/store"
)

const defaultBase = "https://api.openai.com/v1"

// ErrNoKey — chưa có API key trong trang Cấu hình & API.
var ErrNoKey = errors.New("chưa cấu hình API Trực Tiếp (Cấu hình & API)")

// Client — REST client endpoint OpenAI-compatible.
type Client struct {
	Base  string
	Key   string
	Model string
	HTTP  *http.Client
}

// NewFromSettings tạo client từ Settings (openaiBase/openaiKey/openaiModel).
// Base rỗng → https://api.openai.com/v1; tự thêm "/v1" nếu Base chưa kết thúc bằng "/v1".
func NewFromSettings(st *store.Store) *Client {
	cfg := st.Settings()
	return &Client{
		Base:  normalizeBase(cfg.OpenAIBase),
		Key:   strings.TrimSpace(cfg.OpenAIKey),
		Model: strings.TrimSpace(cfg.OpenAIModel),
		HTTP:  &http.Client{Timeout: 180 * time.Second},
	}
}

// normalizeBase chuẩn hóa base URL: bỏ "/" cuối, thêm "/v1" nếu thiếu.
func normalizeBase(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return defaultBase
	}
	base = strings.TrimSuffix(base, "/")
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	return base
}

// ---------- request / response ----------

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// ChatText gọi POST {Base}/chat/completions với system + user message,
// trả về nội dung văn bản của choices[0].
func (c *Client) ChatText(ctx context.Context, system, user string) (string, error) {
	if c.Key == "" {
		return "", ErrNoKey
	}
	if c.Model == "" {
		return "", errors.New("chưa cấu hình model cho API Trực Tiếp (Cấu hình & API)")
	}
	var msgs []chatMessage
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: user})

	body, err := json.Marshal(chatRequest{Model: c.Model, Messages: msgs})
	if err != nil {
		return "", fmt.Errorf("mã hoá request API Trực Tiếp: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("tạo request API Trực Tiếp: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("gọi API Trực Tiếp thất bại: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", fmt.Errorf("đọc response API Trực Tiếp: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API Trực Tiếp lỗi (HTTP %d): %s", resp.StatusCode, apiErrMessage(data))
	}
	var out chatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("giải mã response API Trực Tiếp: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("API Trực Tiếp không trả về lựa chọn nào (choices rỗng)")
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return "", errors.New("API Trực Tiếp không trả về nội dung nào")
	}
	return text, nil
}

// apiErrMessage lấy error.message từ body JSON lỗi; nếu không có,
// trả tối đa 300 ký tự đầu của body.
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
