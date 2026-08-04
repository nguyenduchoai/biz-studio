package veo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/store"
)

const (
	defaultBase = "https://generativelanguage.googleapis.com"
	pollEvery   = 10 * time.Second
	maxPoll     = 12 * time.Minute // Veo thường 1-4 phút; quá mức này coi như hỏng
	maxImageMB  = 18
)

// ErrNoKey — chưa có khoá. Veo tính tiền trên khoá của người dùng nên không có
// đường dự phòng nào: không khoá thì không chạy.
var ErrNoKey = errors.New("chưa cấu hình khoá Veo (Cấu hình & API) — Veo tính phí theo giây trên khoá Google của bạn")

// Client — REST client Veo.
type Client struct {
	Key   string
	Base  string
	Model string
	HTTP  *http.Client
}

// NewFromSettings dựng client từ Settings. Chưa khai khoá Veo riêng thì dùng
// khoá Gemini — cùng một hệ thống khoá của Google, đỡ bắt khai hai lần.
func NewFromSettings(st *store.Store) *Client {
	cfg := st.Settings()
	key := strings.TrimSpace(cfg.VeoAPIKey)
	if key == "" {
		key = strings.TrimSpace(cfg.GeminiAPIKey)
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.GeminiBase), "/")
	if base == "" {
		base = defaultBase
	}
	model := strings.TrimSpace(cfg.VeoModel)
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		Key:   key,
		Base:  base,
		Model: model,
		// Chỉ là thời gian chờ MỘT lượt gọi; cả quá trình chờ theo maxPoll.
		HTTP: &http.Client{Timeout: 120 * time.Second},
	}
}

// Ready — đã đủ điều kiện gọi Veo chưa.
func (c *Client) Ready() bool { return strings.TrimSpace(c.Key) != "" }

// Opts — tham số một lần tạo video.
type Opts struct {
	Prompt     string `json:"prompt"`
	Negative   string `json:"negative"`   // thứ cần tránh
	Aspect     string `json:"aspect"`     // 9:16 | 16:9
	Resolution string `json:"resolution"` // 720p | 1080p | 4k
	Seconds    int    `json:"seconds"`    // 4 | 6 | 8
	ImagePath  string `json:"imagePath"`  // ảnh làm khung hình đầu (tuỳ chọn)
	AllowAdult bool   `json:"allowAdult"` // true = chỉ cho người lớn xuất hiện
}

// ---------- thân request / response ----------

type blob struct {
	InlineData struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"`
	} `json:"inlineData"`
}

type instance struct {
	Prompt string `json:"prompt"`
	Image  *blob  `json:"image,omitempty"`
}

type params struct {
	AspectRatio      string `json:"aspectRatio"`
	Resolution       string `json:"resolution"`
	DurationSeconds  string `json:"durationSeconds"`
	NumberOfVideos   int    `json:"numberOfVideos"`
	PersonGeneration string `json:"personGeneration"`
	NegativePrompt   string `json:"negativePrompt,omitempty"`
}

type predictReq struct {
	Instances  []instance `json:"instances"`
	Parameters params     `json:"parameters"`
}

type operation struct {
	Name  string `json:"name"`
	Done  bool   `json:"done"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Response struct {
		GenerateVideoResponse struct {
			GeneratedSamples []struct {
				Video struct {
					URI      string `json:"uri"`
					MimeType string `json:"mimeType"`
				} `json:"video"`
			} `json:"generatedSamples"`
		} `json:"generateVideoResponse"`
	} `json:"response"`
}

// Generate tạo một clip và ghi ra dst (.mp4). Trả chi phí ước tính đã dùng.
//
// Ba bước: gửi yêu cầu → chờ (Veo chạy nền, thường 1-4 phút) → tải file về.
func (c *Client) Generate(ctx context.Context, o Opts, dst string, upd func(float64, string)) (float64, error) {
	if upd == nil {
		upd = func(float64, string) {}
	}
	if !c.Ready() {
		return 0, ErrNoKey
	}
	if strings.TrimSpace(o.Prompt) == "" {
		return 0, errors.New("chưa có mô tả cảnh cần tạo")
	}
	o.Seconds = normalizeDuration(o.Seconds)
	o.Resolution = normalizeResolution(o.Resolution)
	o.Aspect = normalizeAspect(o.Aspect)
	if err := checkCombo(o.Resolution, o.Seconds); err != nil {
		return 0, err
	}
	cost, _ := EstimateUSD(c.Model, o.Resolution, o.Seconds, 1)

	req := predictReq{
		Instances: []instance{{Prompt: strings.TrimSpace(o.Prompt)}},
		Parameters: params{
			AspectRatio:      o.Aspect,
			Resolution:       o.Resolution,
			DurationSeconds:  fmt.Sprint(o.Seconds),
			NumberOfVideos:   1,
			PersonGeneration: personMode(o.AllowAdult),
			NegativePrompt:   strings.TrimSpace(o.Negative),
		},
	}
	if p := strings.TrimSpace(o.ImagePath); p != "" {
		b, err := imageBlob(p)
		if err != nil {
			return 0, err
		}
		req.Instances[0].Image = b
	}

	upd(5, fmt.Sprintf("Gửi yêu cầu tới Veo (%d giây, %s, ước tính $%.2f)…", o.Seconds, o.Resolution, cost))
	opName, err := c.submit(ctx, req)
	if err != nil {
		return 0, err
	}

	upd(15, "Veo đang dựng video — việc này thường mất 1-4 phút…")
	uri, err := c.wait(ctx, opName, upd)
	if err != nil {
		return 0, err
	}

	upd(90, "Tải video về máy…")
	if err := c.download(ctx, uri, dst); err != nil {
		return 0, err
	}
	upd(99, fmt.Sprintf("Xong — chi phí ước tính $%.2f", cost))
	return cost, nil
}

// submit gửi yêu cầu tạo, trả tên tác vụ chạy nền.
func (c *Client) submit(ctx context.Context, req predictReq) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("mã hoá yêu cầu Veo: %w", err)
	}
	url := fmt.Sprintf("%s/v1beta/models/%s:predictLongRunning", c.Base, c.Model)
	data, err := c.do(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", err
	}
	var op operation
	if err := json.Unmarshal(data, &op); err != nil {
		return "", fmt.Errorf("giải mã phản hồi Veo: %w", err)
	}
	if strings.TrimSpace(op.Name) == "" {
		return "", errors.New("Veo không trả về mã tác vụ — không theo dõi được tiến độ")
	}
	return op.Name, nil
}

// wait hỏi lại tác vụ cho tới khi xong, trả đường dẫn tải video.
func (c *Client) wait(ctx context.Context, opName string, upd func(float64, string)) (string, error) {
	url := fmt.Sprintf("%s/v1beta/%s", c.Base, strings.TrimPrefix(opName, "/"))
	deadline := time.Now().Add(maxPoll)
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollEvery):
		}
		data, err := c.do(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		var op operation
		if err := json.Unmarshal(data, &op); err != nil {
			return "", fmt.Errorf("giải mã trạng thái Veo: %w", err)
		}
		if op.Error != nil && op.Error.Message != "" {
			return "", fmt.Errorf("Veo báo lỗi: %s", op.Error.Message)
		}
		if op.Done {
			s := op.Response.GenerateVideoResponse.GeneratedSamples
			if len(s) == 0 || strings.TrimSpace(s[0].Video.URI) == "" {
				return "", errors.New("Veo báo xong nhưng không trả về video nào — thường do prompt bị chặn bởi bộ lọc nội dung")
			}
			return s[0].Video.URI, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("Veo chạy quá %v mà chưa xong — thử lại hoặc hạ độ phân giải", maxPoll)
		}
		upd(15+float64(i%20)*3, fmt.Sprintf("Veo đang dựng video… (đã chờ %d giây)", (i+1)*int(pollEvery/time.Second)))
	}
}

// download tải file video về dst. Ghi ra file tạm rồi mới đổi tên để mạng đứt
// giữa chừng không để lại file mp4 hỏng.
func (c *Client) download(ctx context.Context, uri, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("tạo thư mục kết quả: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return fmt.Errorf("tạo yêu cầu tải video: %w", err)
	}
	req.Header.Set("x-goog-api-key", c.Key)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("tải video từ Veo thất bại: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return fmt.Errorf("tải video từ Veo lỗi (HTTP %d): %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("tạo file video: %w", err)
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("ghi file video: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// do gửi một lượt HTTP kèm khoá, trả body khi thành công.
func (c *Client) do(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, fmt.Errorf("tạo request Veo: %w", err)
	}
	req.Header.Set("x-goog-api-key", c.Key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gọi Veo API thất bại: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("đọc phản hồi Veo: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Veo API lỗi (HTTP %d): %s", res.StatusCode, errMessage(data, res.StatusCode))
	}
	return data, nil
}

// errMessage rút thông báo lỗi và dịch vài mã hay gặp sang lời khuyên cụ thể.
func errMessage(data []byte, code int) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	msg := strings.TrimSpace(string(data))
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		msg = e.Error.Message
	}
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return msg + " — kiểm tra khoá Veo và xem dự án Google đã bật thanh toán chưa (Veo không có bậc miễn phí)"
	case http.StatusTooManyRequests:
		return msg + " — vượt hạn mức gọi, chờ ít phút rồi thử lại"
	case http.StatusNotFound:
		return msg + " — có thể model này đã ngừng phục vụ, chọn model khác trong Cấu hình & API"
	}
	if len(msg) > 400 {
		msg = msg[:400] + "…"
	}
	return msg
}

// personMode — Veo nhận allow_all hoặc allow_adult.
func personMode(allowAdult bool) string {
	if allowAdult {
		return "allow_adult"
	}
	return "allow_all"
}

// imageBlob đọc ảnh khung hình đầu thành dữ liệu nhúng.
func imageBlob(path string) (*blob, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("không đọc được ảnh khung hình đầu: %w", err)
	}
	if st.Size() > maxImageMB<<20 {
		return nil, fmt.Errorf("ảnh khung hình đầu quá lớn (%.1f MB, tối đa %d MB)", float64(st.Size())/(1<<20), maxImageMB)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("không đọc được ảnh khung hình đầu: %w", err)
	}
	mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if !strings.HasPrefix(mt, "image/") {
		mt = "image/png"
	}
	b := &blob{}
	b.InlineData.MimeType = mt
	b.InlineData.Data = base64.StdEncoding.EncodeToString(raw)
	return b, nil
}
