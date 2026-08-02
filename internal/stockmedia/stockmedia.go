// Package stockmedia — "Media Xu hướng": tìm và tải ảnh stock theo từ khóa
// từ Pexels API (https://api.pexels.com/v1/search).
package stockmedia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"bizstudio/internal/store"
)

const searchURL = "https://api.pexels.com/v1/search"

// ErrNoKey — chưa có Pexels API key trong trang Cấu hình & API.
var ErrNoKey = errors.New("chưa cấu hình Pexels API key (Media Xu hướng)")

var httpClient = &http.Client{Timeout: 60 * time.Second}

type pexelsPhoto struct {
	Src struct {
		Original string `json:"original"`
		Large2x  string `json:"large2x"`
	} `json:"src"`
}

type pexelsResponse struct {
	Photos []pexelsPhoto `json:"photos"`
}

// SearchImage tìm ảnh Pexels theo từ khóa (orientation suy từ w/h: dọc, ngang
// hoặc vuông) rồi tải ảnh đầu tiên (large2x, fallback original) về dst.
func SearchImage(ctx context.Context, st *store.Store, keyword string, w, h int, dst string) error {
	key := strings.TrimSpace(st.Settings().PexelsKey)
	if key == "" {
		return ErrNoKey
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return errors.New("từ khóa tìm ảnh stock trống")
	}

	photo, err := search(ctx, key, keyword, orientation(w, h))
	if err != nil {
		return err
	}
	src := strings.TrimSpace(photo.Src.Large2x)
	if src == "" {
		src = strings.TrimSpace(photo.Src.Original)
	}
	if src == "" {
		return fmt.Errorf("ảnh stock cho '%s' không có đường dẫn tải về", keyword)
	}
	return download(ctx, src, dst)
}

// orientation suy hướng ảnh từ kích thước khung hình đích.
func orientation(w, h int) string {
	switch {
	case h > w:
		return "portrait"
	case w > h:
		return "landscape"
	default:
		return "square"
	}
}

// search gọi Pexels search API, trả photo đầu tiên trong kết quả.
func search(ctx context.Context, key, keyword, orient string) (*pexelsPhoto, error) {
	q := url.Values{}
	q.Set("query", keyword)
	q.Set("orientation", orient)
	q.Set("per_page", "3")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("tạo request Pexels: %w", err)
	}
	req.Header.Set("Authorization", key)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gọi Pexels API thất bại: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("đọc response Pexels: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Pexels API lỗi (HTTP %d): %s", resp.StatusCode, bodySnippet(data))
	}
	var out pexelsResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("giải mã response Pexels: %w", err)
	}
	if len(out.Photos) == 0 {
		return nil, fmt.Errorf("không tìm thấy ảnh stock cho '%s'", keyword)
	}
	return &out.Photos[0], nil
}

// download tải ảnh từ src về file dst.
func download(ctx context.Context, src, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return fmt.Errorf("tạo request tải ảnh: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("tải ảnh stock thất bại: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tải ảnh stock thất bại (HTTP %d)", resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("không tạo được file %s: %w", dst, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("ghi file ảnh stock thất bại: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("đóng file ảnh stock thất bại: %w", err)
	}
	return nil
}

// bodySnippet lấy error/message JSON nếu có, else tối đa 300 ký tự đầu body.
func bodySnippet(data []byte) string {
	var e struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error != "" {
		return e.Error
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
