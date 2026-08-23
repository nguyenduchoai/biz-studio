// Package artifact lưu lại kết quả của những bước đắt tiền để lần sau khỏi làm
// lại.
//
// Hai chỗ đau cụ thể:
//
//   - Bóc băng một video 20 phút mất vài phút. Chạy "Rút clip ngắn" rồi chạy
//     "Hợp tuyển theo chủ đề" trên CÙNG một video là bóc băng hai lần, y hệt
//     nhau, không được gì.
//   - Chấm điểm một video 2 tiếng là 19 lượt gọi AI có tính tiền. Job chết ở
//     bước sau là 19 lượt đó mất trắng.
//
// Khoá tính từ MỌI thứ ảnh hưởng tới kết quả — kể cả mốc sửa đổi và kích thước
// file nguồn. Người dùng thay file rồi giữ nguyên tên là chuyện thường; khoá chỉ
// theo tên là trả về kết quả của file cũ, một kiểu hỏng cực khó lần ra.
package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxAge — quá hạn này thì dọn. Bản bóc băng của một video đã xoá từ lâu không
// còn ai cần, mà vẫn chiếm chỗ mãi.
const maxAge = 30 * 24 * time.Hour

// Store — thư mục chứa cache.
type Store struct{ dir string }

func New(dataDir string) *Store {
	d := filepath.Join(dataDir, "cache")
	_ = os.MkdirAll(d, 0o755)
	return &Store{dir: d}
}

// Key băm mọi phần ảnh hưởng tới kết quả thành một khoá ngắn.
func Key(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])[:20]
}

// FileKey mô tả một file nguồn: đường dẫn + mốc sửa đổi + kích thước.
//
// Không dùng nội dung file để băm: video vài GB thì băm còn lâu hơn cả bóc
// băng. Bộ ba này đủ để phát hiện file đã đổi trong mọi tình huống thực tế.
func FileKey(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return path + "|?"
	}
	return fmt.Sprintf("%s|%d|%d", path, fi.ModTime().UnixNano(), fi.Size())
}

// Load đọc kết quả đã lưu vào v. Trả false nếu chưa có hoặc file hỏng.
func (s *Store) Load(kind, key string, v any) bool {
	if s == nil {
		return false
	}
	raw, err := os.ReadFile(s.path(kind, key))
	if err != nil {
		return false
	}
	if json.Unmarshal(raw, v) != nil {
		// File hỏng (ghi dở vì mất điện chẳng hạn) — bỏ đi, tính như chưa có.
		_ = os.Remove(s.path(kind, key))
		return false
	}
	return true
}

// Save ghi kết quả. Lỗi ghi KHÔNG được làm hỏng job đang chạy: cache là thứ
// tăng tốc, không phải thứ bắt buộc.
func (s *Store) Save(kind, key string, v any) {
	if s == nil {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	p := s.path(kind, key)
	// Ghi ra file tạm rồi đổi tên: mất điện giữa chừng thì để lại file tạm chứ
	// không để lại một cache hỏng mà lần sau vẫn tưởng là dùng được.
	tmp := p + ".tmp"
	if os.WriteFile(tmp, raw, 0o644) != nil {
		return
	}
	if os.Rename(tmp, p) != nil {
		_ = os.Remove(tmp)
	}
}

func (s *Store) path(kind, key string) string {
	return filepath.Join(s.dir, kind+"-"+key+".json")
}

// Prune xoá cache quá hạn. Trả số file đã xoá.
func (s *Store) Prune() int {
	if s == nil {
		return 0
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0
	}
	cut := time.Now().Add(-maxAge)
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cut) {
			continue
		}
		if os.Remove(filepath.Join(s.dir, e.Name())) == nil {
			n++
		}
	}
	return n
}
