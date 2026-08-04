package htmlvideo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------- Font tiếng Việt dùng chung cho mọi máy ----------
//
// Vì sao cần: bộ font mặc định phụ thuộc hệ điều hành (-apple-system trên máy
// Mac, Segoe UI trên Windows, tuỳ máy trên Linux). Hệ quả là (1) cùng một dự án
// render ra hai máy cho ra hai kiểu chữ khác nhau, và (2) máy nào không có font
// đủ dấu tiếng Việt thì chữ hiện ra ô vuông.
//
// Be Vietnam Pro là font thiết kế riêng cho tiếng Việt, giấy phép SIL Open Font
// (cho phép dùng và phát hành kèm). Tải một lần về data/fonts rồi nhúng thẳng
// vào trang cảnh, không phụ thuộc font máy nữa.

// VietFontFamily — tên họ font để đặt đầu chuỗi font.
const VietFontFamily = `"Be Vietnam Pro"`

const vietFontRepo = "https://raw.githubusercontent.com/google/fonts/main/ofl/bevietnampro/"

// vietFontFiles — chỉ lấy 3 độ đậm thực sự dùng: chữ thường, bán đậm, rất đậm.
var vietFontFiles = []struct {
	Weight int
	File   string
}{
	{400, "BeVietnamPro-Regular.ttf"},
	{600, "BeVietnamPro-SemiBold.ttf"},
	{800, "BeVietnamPro-ExtraBold.ttf"},
}

// FontsDir — thư mục chứa font trong data dir.
func FontsDir(dataDir string) string { return filepath.Join(dataDir, "fonts") }

// VietFontReady kiểm tra đã có đủ font chưa (không gọi mạng).
func VietFontReady(dataDir string) bool {
	dir := FontsDir(dataDir)
	for _, f := range vietFontFiles {
		st, err := os.Stat(filepath.Join(dir, f.File))
		if err != nil || st.Size() < 1024 {
			return false
		}
	}
	return true
}

// EnsureVietFont tải font về nếu còn thiếu. Đã đủ thì không gọi mạng.
func EnsureVietFont(ctx context.Context, dataDir string) error {
	dir := FontsDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("tạo thư mục font: %w", err)
	}
	cli := &http.Client{Timeout: 60 * time.Second}
	for _, f := range vietFontFiles {
		dst := filepath.Join(dir, f.File)
		if st, err := os.Stat(dst); err == nil && st.Size() >= 1024 {
			continue
		}
		if err := downloadTo(ctx, cli, vietFontRepo+f.File, dst); err != nil {
			return fmt.Errorf("tải font %s thất bại: %w", f.File, err)
		}
	}
	return nil
}

// downloadTo tải một file về đường dẫn tạm rồi mới đổi tên, tránh để lại file
// dở dang khi mạng đứt giữa chừng.
func downloadTo(ctx context.Context, cli *http.Client, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("máy chủ trả mã %d", res.StatusCode)
	}
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, res.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// FontFaceCSS trả các khối @font-face trỏ tới font đã tải. Chưa tải thì trả
// chuỗi rỗng — trang vẫn render bình thường bằng font hệ điều hành.
func FontFaceCSS(dataDir string) string {
	if !VietFontReady(dataDir) {
		return ""
	}
	dir := FontsDir(dataDir)
	var b strings.Builder
	for _, f := range vietFontFiles {
		fmt.Fprintf(&b, "@font-face{font-family:%s;font-style:normal;font-weight:%d;src:url(%q) format(\"truetype\");}\n",
			VietFontFamily, f.Weight, fileURL(filepath.Join(dir, f.File)))
	}
	return b.String()
}

// WithVietFont đặt Be Vietnam Pro lên đầu chuỗi font (nếu đã tải), giữ nguyên
// phần còn lại làm dự phòng.
func WithVietFont(stack, dataDir string) string {
	if !VietFontReady(dataDir) || strings.Contains(stack, "Be Vietnam Pro") {
		return stack
	}
	if strings.TrimSpace(stack) == "" {
		return VietFontFamily
	}
	return VietFontFamily + ", " + stack
}
