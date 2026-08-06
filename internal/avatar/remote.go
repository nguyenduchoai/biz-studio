package avatar

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------- chế độ remote: đẩy việc sang máy GPU ----------
//
// Máy GPU chạy scripts/longcat-worker.py. Giao thức cố ý làm đơn giản nhất có
// thể (JSON + base64, không multipart) để worker chỉ cần thư viện chuẩn của
// Python — máy GPU vốn đã phải cài cả đống thứ cho model rồi.

const (
	remotePoll     = 8 * time.Second
	remoteMaxWait  = 30 * time.Minute
	remoteHTTPWait = 5 * time.Minute
)

type genReq struct {
	Prompt   string `json:"prompt"`
	ImageB64 string `json:"image_b64"`
	ImageExt string `json:"image_ext"`
	AudioB64 string `json:"audio_b64"`
	AudioExt string `json:"audio_ext"`
}

type genResp struct {
	JobID string `json:"job_id"`
	Error string `json:"error"`
}

type statusResp struct {
	State    string  `json:"state"` // queued | running | done | error
	Progress float64 `json:"progress"`
	Detail   string  `json:"detail"`
	Error    string  `json:"error"`
}

type healthResp struct {
	OK         bool   `json:"ok"`
	GPU        string `json:"gpu"`
	Checkpoint bool   `json:"checkpoint"`
	Busy       bool   `json:"busy"`
	Detail     string `json:"detail"`
}

func httpClient() *http.Client { return &http.Client{Timeout: remoteHTTPWait} }

// checkRemote hỏi máy GPU xem có sẵn sàng không.
func checkRemote(ctx context.Context, c cfg) (bool, string) {
	if c.worker == "" {
		return false, "Chưa khai địa chỉ máy GPU (vd: http://192.168.1.50:7070)"
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.worker+"/health", nil)
	if err != nil {
		return false, "Địa chỉ máy GPU không hợp lệ: " + c.worker
	}
	res, err := httpClient().Do(req)
	if err != nil {
		return false, "Không kết nối được máy GPU " + c.worker + " — kiểm tra máy đó đã chạy scripts/longcat-worker.py chưa"
	}
	defer res.Body.Close()
	var h healthResp
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&h) != nil {
		return false, "Máy ở " + c.worker + " trả về dữ liệu lạ — có đúng là longcat-worker không?"
	}
	if !h.OK {
		return false, "Máy GPU chưa sẵn sàng: " + h.Detail
	}
	busy := ""
	if h.Busy {
		busy = " · đang bận một việc khác"
	}
	return true, "Sẵn sàng · " + h.GPU + busy
}

// generateRemote gửi ảnh + giọng sang máy GPU, chờ xong rồi tải video về.
func generateRemote(ctx context.Context, c cfg, o Opts, dst string, upd func(float64, string)) error {
	if c.worker == "" {
		return fmt.Errorf("chưa khai địa chỉ máy GPU")
	}
	upd(5, "Gửi ảnh và giọng sang máy GPU…")
	img, err := os.ReadFile(o.ImagePath)
	if err != nil {
		return fmt.Errorf("đọc ảnh nhân vật: %w", err)
	}
	aud, err := os.ReadFile(o.AudioPath)
	if err != nil {
		return fmt.Errorf("đọc file giọng: %w", err)
	}
	body, err := json.Marshal(genReq{
		Prompt:   promptOf(o.Prompt),
		ImageB64: base64.StdEncoding.EncodeToString(img),
		ImageExt: strings.ToLower(filepath.Ext(o.ImagePath)),
		AudioB64: base64.StdEncoding.EncodeToString(aud),
		AudioExt: strings.ToLower(filepath.Ext(o.AudioPath)),
	})
	if err != nil {
		return fmt.Errorf("mã hoá yêu cầu: %w", err)
	}

	var g genResp
	if err := postJSON(ctx, c.worker+"/generate", body, &g); err != nil {
		return err
	}
	if g.Error != "" {
		return fmt.Errorf("máy GPU từ chối: %s", g.Error)
	}
	if g.JobID == "" {
		return fmt.Errorf("máy GPU không trả về mã việc")
	}

	upd(15, "Máy GPU đang dựng video…")
	if err := waitRemote(ctx, c, g.JobID, upd); err != nil {
		return err
	}
	upd(92, "Tải video về máy…")
	return downloadResult(ctx, c.worker+"/result/"+g.JobID, dst)
}

// waitRemote hỏi trạng thái tới khi xong.
func waitRemote(ctx context.Context, c cfg, jobID string, upd func(float64, string)) error {
	deadline := time.Now().Add(remoteMaxWait)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(remotePoll):
		}
		var s statusResp
		if err := getJSON(ctx, c.worker+"/status/"+jobID, &s); err != nil {
			return err
		}
		switch s.State {
		case "done":
			return nil
		case "error":
			return fmt.Errorf("máy GPU báo lỗi: %s", firstNonEmptyStr(s.Error, s.Detail, "không rõ nguyên nhân"))
		}
		// Tiến độ của worker nằm trong 0..100; nén về khoảng 15..90 của job này.
		p := 15 + s.Progress*0.75
		upd(p, firstNonEmptyStr(s.Detail, "Máy GPU đang dựng video…"))
		if time.Now().After(deadline) {
			return fmt.Errorf("máy GPU chạy quá %v mà chưa xong", remoteMaxWait)
		}
	}
}

func postJSON(ctx context.Context, url string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("tạo yêu cầu tới máy GPU: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return doJSON(req, out)
}

func getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("tạo yêu cầu tới máy GPU: %w", err)
	}
	return doJSON(req, out)
}

func doJSON(req *http.Request, out any) error {
	res, err := httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("gọi máy GPU thất bại: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("đọc phản hồi máy GPU: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("máy GPU trả mã %d: %s", res.StatusCode, tailStr(string(data), 300))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("phản hồi máy GPU không đúng định dạng: %w", err)
	}
	return nil
}

// downloadResult tải video về, ghi tạm rồi mới đổi tên để mạng đứt giữa chừng
// không để lại file mp4 hỏng.
func downloadResult(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("tạo yêu cầu tải video: %w", err)
	}
	res, err := (&http.Client{Timeout: 20 * time.Minute}).Do(req)
	if err != nil {
		return fmt.Errorf("tải video từ máy GPU thất bại: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return fmt.Errorf("tải video lỗi (mã %d): %s", res.StatusCode, strings.TrimSpace(string(b)))
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
