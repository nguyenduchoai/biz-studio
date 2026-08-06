package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// ---------- Nạp danh sách model từ chính API ----------
//
// Vì sao không gõ tay: danh sách model của Google đổi liên tục — model mới ra,
// model preview bị gỡ. Gõ tay hoặc để sẵn một danh sách trong mã nguồn thì chỉ
// đúng vào ngày viết, sau đó người dùng chọn phải model không còn tồn tại và
// nhận lỗi 404 mà không hiểu vì sao.

// ModelInfo — một model kèm khả năng của nó.
type ModelInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Methods     []string `json:"methods"`
	InputLimit  int      `json:"inputLimit"`
	OutputLimit int      `json:"outputLimit"`
}

// ModelGroups — model đã chia theo việc, để giao diện đổ đúng ô.
type ModelGroups struct {
	Text  []ModelInfo `json:"text"`  // sinh văn bản (tách cảnh, dịch, viết kịch bản)
	Image []ModelInfo `json:"image"` // sinh ảnh (storyboard, thumbnail)
	TTS   []ModelInfo `json:"tts"`   // đọc giọng
	Video []ModelInfo `json:"video"` // sinh video (Veo)
	Total int         `json:"total"`
}

type listResp struct {
	Models []struct {
		Name                       string   `json:"name"`
		DisplayName                string   `json:"displayName"`
		SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		InputTokenLimit            int      `json:"inputTokenLimit"`
		OutputTokenLimit           int      `json:"outputTokenLimit"`
	} `json:"models"`
	NextPageToken string `json:"nextPageToken"`
}

// ListModels hỏi Google danh sách model dùng được với khoá hiện tại.
// Đây là lệnh ĐỌC, không sinh nội dung nên không tốn tiền.
func (c *Client) ListModels(ctx context.Context) (ModelGroups, error) {
	var out ModelGroups
	if c.Key == "" {
		return out, ErrNoKey
	}
	var all []ModelInfo
	page := ""
	// Google trả theo trang; lấy hết chứ không dừng ở trang đầu — model mới hay
	// nằm cuối danh sách.
	for i := 0; i < 10; i++ {
		url := fmt.Sprintf("%s/v1beta/models?key=%s&pageSize=200", c.Base, c.Key)
		if page != "" {
			url += "&pageToken=" + page
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return out, fmt.Errorf("tạo request danh sách model: %w", err)
		}
		res, err := c.HTTP.Do(req)
		if err != nil {
			return out, fmt.Errorf("gọi Gemini API thất bại: %w", err)
		}
		data, rerr := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		res.Body.Close()
		if rerr != nil {
			return out, fmt.Errorf("đọc danh sách model: %w", rerr)
		}
		if res.StatusCode != http.StatusOK {
			return out, fmt.Errorf("Gemini API lỗi (HTTP %d): %s", res.StatusCode, apiErrMessage(data))
		}
		var lr listResp
		if err := json.Unmarshal(data, &lr); err != nil {
			return out, fmt.Errorf("giải mã danh sách model: %w", err)
		}
		for _, m := range lr.Models {
			all = append(all, ModelInfo{
				ID:          strings.TrimPrefix(m.Name, "models/"),
				Name:        m.DisplayName,
				Methods:     m.SupportedGenerationMethods,
				InputLimit:  m.InputTokenLimit,
				OutputLimit: m.OutputTokenLimit,
			})
		}
		if lr.NextPageToken == "" {
			break
		}
		page = lr.NextPageToken
	}

	out.Total = len(all)
	for _, m := range all {
		switch {
		case has(m.Methods, "predictLongRunning"):
			out.Video = append(out.Video, m)
		case isTTS(m):
			out.TTS = append(out.TTS, m)
		case isImage(m):
			out.Image = append(out.Image, m)
		case has(m.Methods, "generateContent"):
			out.Text = append(out.Text, m)
		}
	}
	sortNewestFirst(out.Text)
	sortNewestFirst(out.Image)
	sortNewestFirst(out.TTS)
	sortNewestFirst(out.Video)
	return out, nil
}

// isTTS / isImage: API không có cờ riêng cho hai loại này, chỉ phân biệt được
// qua tên model — nên nhận diện theo tên, và xét TTS trước vì model TTS cũng
// khai generateContent như model văn bản.
func isTTS(m ModelInfo) bool { return strings.Contains(m.ID, "tts") }

func isImage(m ModelInfo) bool {
	return strings.Contains(m.ID, "image") || strings.HasPrefix(m.ID, "imagen-")
}

func has(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// sortNewestFirst xếp model theo thứ tự hữu ích cho người chọn:
//
//  1. Dòng model — gemini/veo trước, imagen sau, gemma và model lạ cuối.
//     So số phiên bản giữa hai dòng khác nhau là vô nghĩa: gemma-4 không hề
//     mới hơn hay mạnh hơn gemini-3.6, chỉ là đánh số riêng.
//  2. Bí danh "-latest" lên đầu dòng của nó — đây là lựa chọn an toàn nhất cho
//     người không muốn theo dõi model mới, vì Google tự trỏ sang bản mới.
//  3. Số phiên bản giảm dần, bản chính thức trước bản preview.
func sortNewestFirst(ms []ModelInfo) {
	sort.SliceStable(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if fa, fb := familyRank(a.ID), familyRank(b.ID); fa != fb {
			return fa > fb
		}
		if la, lb := strings.HasSuffix(a.ID, "-latest"), strings.HasSuffix(b.ID, "-latest"); la != lb {
			return la
		}
		if va, vb := versionScore(a.ID), versionScore(b.ID); va != vb {
			return va > vb
		}
		if pa, pb := strings.Contains(a.ID, "preview"), strings.Contains(b.ID, "preview"); pa != pb {
			return !pa
		}
		return a.ID < b.ID
	})
}

// familyRank — dòng model nào nên hiện trước.
func familyRank(id string) int {
	switch {
	case strings.HasPrefix(id, "gemini-"), strings.HasPrefix(id, "veo-"):
		return 3
	case strings.HasPrefix(id, "imagen-"):
		return 2
	case strings.HasPrefix(id, "gemma-"):
		return 1
	default:
		return 0
	}
}

// versionScore rút số phiên bản trong tên model (gemini-3.6-flash → 360).
// Không có số thì về 0 và nằm cuối danh sách.
func versionScore(id string) int {
	digits := ""
	for i := 0; i < len(id); i++ {
		ch := id[i]
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
			// lấy tiếp phần thập phân nếu có (3.6 → "36")
			for i+1 < len(id) && (id[i+1] == '.' || (id[i+1] >= '0' && id[i+1] <= '9')) {
				i++
				if id[i] != '.' {
					digits += string(id[i])
				}
			}
			break
		}
	}
	if digits == "" {
		return 0
	}
	// chuẩn hoá về 3 chữ số: "3"→300, "36"→360, "40"→400
	for len(digits) < 3 {
		digits += "0"
	}
	n := 0
	for i := 0; i < 3; i++ {
		n = n*10 + int(digits[i]-'0')
	}
	return n
}
