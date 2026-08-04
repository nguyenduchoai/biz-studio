package whisper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Word — một từ kèm mốc thời gian (giây).
type Word struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Segment — một câu/cụm câu, kèm danh sách từ bên trong.
type Segment struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Words []Word  `json:"words"`
}

// Transcript — kết quả bóc băng đầy đủ.
type Transcript struct {
	Language string    `json:"language"`
	Duration float64   `json:"duration"`
	Segments []Segment `json:"segments"`
}

// Words gom mọi từ của mọi segment theo thứ tự thời gian.
// Segment không có mốc từng từ (hiếm) được coi như MỘT từ dài để vẫn được bảo vệ.
func (t *Transcript) Words() []Word {
	if t == nil {
		return nil
	}
	var out []Word
	for _, s := range t.Segments {
		if len(s.Words) == 0 {
			if s.End > s.Start {
				out = append(out, Word{Text: s.Text, Start: s.Start, End: s.End})
			}
			continue
		}
		out = append(out, s.Words...)
	}
	return out
}

// WordCount — tổng số từ có mốc.
func (t *Transcript) WordCount() int { return len(t.Words()) }

// Text ghép toàn bộ nội dung đã bóc.
func (t *Transcript) Text() string {
	if t == nil {
		return ""
	}
	out := ""
	for i, s := range t.Segments {
		if i > 0 {
			out += " "
		}
		out += s.Text
	}
	return out
}

// SaveJSON ghi transcript ra file (tự tạo thư mục cha).
func SaveJSON(tr *Transcript, path string) error {
	if tr == nil {
		return fmt.Errorf("không có transcript để ghi")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("không tạo được thư mục %s: %w", dir, err)
		}
	}
	b, err := json.MarshalIndent(tr, "", " ")
	if err != nil {
		return fmt.Errorf("đóng gói transcript: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("ghi transcript %s: %w", path, err)
	}
	return nil
}

// LoadJSON đọc transcript đã lưu.
func LoadJSON(path string) (*Transcript, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("không đọc được file transcript %s: %w", path, err)
	}
	var tr Transcript
	if err := json.Unmarshal(b, &tr); err != nil {
		return nil, fmt.Errorf("file transcript %s không đúng định dạng JSON: %w", path, err)
	}
	if len(tr.Segments) == 0 {
		return nil, fmt.Errorf("file transcript %s không có đoạn nào", path)
	}
	return &tr, nil
}
