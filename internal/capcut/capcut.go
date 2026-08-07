// Package capcut — sinh dự án CapCut/JianYing (.draft) để người dùng mở tiếp
// trong trình dựng họ đã quen tay: Biz Studio làm phần nặng (cắt cảnh, lời dẫn,
// giọng đọc, phụ đề), tinh chỉnh phát cuối làm ở CapCut.
//
// Định dạng draft là JSON thuần do cộng đồng mổ ngược, KHÔNG được nhà phát
// hành tài liệu hoá hay cam kết: một thư mục chứa draft_content.json (timeline)
// + draft_meta_info.json (metadata), thời gian tính bằng MICRO GIÂY, media
// tham chiếu bằng đường dẫn tuyệt đối trên máy đang mở. Bản CapCut quốc tế
// hiện đọc JSON thuần; bản Trung Quốc đời mới đã mã hoá khi tự lưu lại.
//
// Nguyên tắc của bộ sinh: CHỈ SINH MỚI, không đọc-sửa draft có sẵn — đọc-sửa
// là tự chuốc lấy mọi biến thể phiên bản của định dạng không cam kết này.
package capcut

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/*.json
var tplFS embed.FS

// Draft — một dự án đang được lắp.
type Draft struct {
	content map[string]any
	tracks  map[string]map[string]any // type → track object (video/audio/text mỗi loại một track chính)
	extraAu []map[string]any          // các track audio phụ (lời tràn cảnh nằm đây)
	name    string
	maxEnd  int64 // µs — mốc kết thúc xa nhất, thành duration của draft
}

// Micro đổi giây → micro giây (đơn vị thời gian của định dạng draft).
func Micro(sec float64) int64 { return int64(sec*1e6 + 0.5) }

// New tạo draft trống với khung hình và fps cho trước.
func New(name string, width, height, fps int) (*Draft, error) {
	content, err := loadTpl("content-skeleton.json")
	if err != nil {
		return nil, err
	}
	if fps <= 0 {
		fps = 30
	}
	content["id"] = uuidHex()
	content["name"] = name
	content["fps"] = float64(fps)
	content["canvas_config"] = map[string]any{"width": width, "height": height, "ratio": "original"}
	now := time.Now().Unix()
	if _, ok := content["create_time"]; ok {
		content["create_time"] = now
	}
	if _, ok := content["update_time"]; ok {
		content["update_time"] = now
	}
	return &Draft{
		content: content,
		tracks:  map[string]map[string]any{},
		name:    strings.TrimSpace(name),
	}, nil
}

// AddVideo thêm một khúc của file video nguồn vào track hình.
// srcStart/srcDur là vị trí trong FILE NGUỒN; tgtStart là vị trí trên timeline.
// mediaDur/w/h là thông số của file nguồn (đo bằng ffprobe từ phía gọi).
func (d *Draft) AddVideo(path string, srcStart, srcDur, tgtStart int64, mediaDur int64, w, h int) error {
	mat, err := loadTpl("mat-video.json")
	if err != nil {
		return err
	}
	matID := uuidHex()
	mat["id"] = matID
	mat["path"] = path
	mat["media_path"] = path
	mat["material_name"] = filepath.Base(path)
	mat["duration"] = mediaDur
	mat["width"] = w
	mat["height"] = h
	d.appendMaterial("videos", mat)

	seg, err := loadTpl("seg-video.json")
	if err != nil {
		return err
	}
	spID, err := d.addSpeed()
	if err != nil {
		return err
	}
	seg["id"] = uuidHex()
	seg["material_id"] = matID
	seg["source_timerange"] = timerange(srcStart, srcDur)
	seg["target_timerange"] = timerange(tgtStart, srcDur)
	seg["extra_material_refs"] = []any{spID}
	return d.appendSegment("video", seg, tgtStart+srcDur)
}

// AddAudio thêm một file tiếng vào track lời. overflowTrack=true đặt vào track
// lời PHỤ — dùng khi lời dài hơn cảnh: hai lời liên tiếp sẽ đè giờ lên nhau,
// cùng track thì CapCut không chứa nổi, tách track thì người dùng tự xếp lại.
func (d *Draft) AddAudio(path string, tgtStart, dur int64, mediaDur int64, overflowTrack bool) error {
	mat, err := loadTpl("mat-audio.json")
	if err != nil {
		return err
	}
	matID := uuidHex()
	mat["id"] = matID
	mat["path"] = path
	mat["name"] = filepath.Base(path)
	mat["duration"] = mediaDur
	d.appendMaterial("audios", mat)

	seg, err := loadTpl("seg-audio.json")
	if err != nil {
		return err
	}
	spID, err := d.addSpeed()
	if err != nil {
		return err
	}
	seg["id"] = uuidHex()
	seg["material_id"] = matID
	seg["source_timerange"] = timerange(0, dur)
	seg["target_timerange"] = timerange(tgtStart, dur)
	seg["extra_material_refs"] = []any{spID}

	key := "audio"
	if overflowTrack {
		key = "audio-tran"
	}
	return d.appendSegment(key, seg, tgtStart+dur)
}

// AddText thêm một dòng phụ đề vào track chữ.
func (d *Draft) AddText(text string, tgtStart, dur int64) error {
	content, err := loadTpl("text-content.json")
	if err != nil {
		return err
	}
	content["text"] = text
	if styles, ok := content["styles"].([]any); ok && len(styles) > 0 {
		if st, ok := styles[0].(map[string]any); ok {
			st["range"] = []any{0, len([]rune(text))}
		}
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("mã hoá nội dung chữ: %w", err)
	}

	mat, err := loadTpl("mat-text.json")
	if err != nil {
		return err
	}
	matID := uuidHex()
	mat["id"] = matID
	mat["content"] = string(raw)
	d.appendMaterial("texts", mat)

	seg, err := loadTpl("seg-text.json")
	if err != nil {
		return err
	}
	seg["id"] = uuidHex()
	seg["material_id"] = matID
	seg["target_timerange"] = timerange(tgtStart, dur)
	// Không gắn ref phụ nào: bản sinh ra không được phép có ref trỏ vào hư không.
	seg["extra_material_refs"] = []any{}
	return d.appendSegment("text", seg, tgtStart+dur)
}

// Save ghi thư mục <dir>/<name>/draft_content.json + draft_meta_info.json.
func (d *Draft) Save(dir string) (string, error) {
	if d.name == "" {
		return "", fmt.Errorf("draft chưa có tên")
	}
	folder := filepath.Join(dir, d.name)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", fmt.Errorf("tạo thư mục draft: %w", err)
	}

	d.content["duration"] = d.maxEnd
	// Track lắp theo thứ tự cố định: hình → lời → lời tràn → chữ.
	var tracks []any
	for _, key := range []string{"video", "audio", "audio-tran", "text"} {
		if t, ok := d.tracks[key]; ok {
			tracks = append(tracks, t)
		}
	}
	d.content["tracks"] = tracks

	if err := writeJSON(filepath.Join(folder, "draft_content.json"), d.content); err != nil {
		return "", err
	}

	meta, err := loadTpl("meta-skeleton.json")
	if err != nil {
		return "", err
	}
	meta["draft_id"] = strings.ToUpper(uuidDashed())
	meta["draft_name"] = d.name
	meta["draft_fold_path"] = folder
	if _, ok := meta["draft_root_path"]; ok {
		meta["draft_root_path"] = dir
	}
	if err := writeJSON(filepath.Join(folder, "draft_meta_info.json"), meta); err != nil {
		return "", err
	}
	return folder, nil
}

// ---------- phần ruột ----------

func (d *Draft) addSpeed() (string, error) {
	sp, err := loadTpl("speed.json")
	if err != nil {
		return "", err
	}
	id := uuidHex()
	sp["id"] = id
	sp["speed"] = 1.0
	d.appendMaterial("speeds", sp)
	return id, nil
}

func (d *Draft) appendMaterial(kind string, m map[string]any) {
	mats := d.content["materials"].(map[string]any)
	arr, _ := mats[kind].([]any)
	mats[kind] = append(arr, m)
}

func (d *Draft) appendSegment(trackKey string, seg map[string]any, endUs int64) error {
	t, ok := d.tracks[trackKey]
	if !ok {
		var err error
		t, err = loadTpl("track.json")
		if err != nil {
			return err
		}
		t["id"] = uuidHex()
		// "audio-tran" vẫn là track type audio — key chỉ để tách hai track.
		typ := trackKey
		if trackKey == "audio-tran" {
			typ = "audio"
		}
		t["type"] = typ
		t["name"] = typ
		t["segments"] = []any{}
		d.tracks[trackKey] = t
	}
	segs, _ := t["segments"].([]any)
	seg["render_index"] = len(segs)
	t["segments"] = append(segs, seg)
	if endUs > d.maxEnd {
		d.maxEnd = endUs
	}
	return nil
}

func timerange(start, dur int64) map[string]any {
	return map[string]any{"start": start, "duration": dur}
}

// loadTpl đọc một khuôn JSON thành map MỚI (mỗi lần một bản, không dùng chung).
func loadTpl(name string) (map[string]any, error) {
	raw, err := tplFS.ReadFile("templates/" + name)
	if err != nil {
		return nil, fmt.Errorf("thiếu khuôn %s: %w", name, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("khuôn %s hỏng: %w", name, err)
	}
	return m, nil
}

func writeJSON(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("mã hoá %s: %w", filepath.Base(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("ghi %s: %w", filepath.Base(path), err)
	}
	return os.Rename(tmp, path)
}

func uuidHex() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func uuidDashed() string {
	h := uuidHex()
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
