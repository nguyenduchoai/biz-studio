package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"bizstudio/internal/store"
)

// BuildPrompt dựng prompt tiếng Việt cho Claude CLI từ thông tin dự án.
// version = số phiên hiện có + 1 (đặt tên file output outputs/<id>-v<N>.mp4).
func BuildPrompt(p store.Project, assets []store.Asset, extra string, version int) string {
	w, h, fps := p.Width, p.Height, p.FPS
	if w <= 0 || h <= 0 {
		w, h = 1080, 1920
	}
	if fps <= 0 {
		fps = 30
	}

	var b strings.Builder
	b.WriteString("Bạn là video editor chuyên nghiệp làm việc bằng ffmpeg/ffprobe trong thư mục dự án hiện tại.\n\n")

	b.WriteString("## Thông số dự án\n")
	fmt.Fprintf(&b, "- Tên dự án: %s\n", p.Name)
	fmt.Fprintf(&b, "- Kích thước video: %dx%d\n", w, h)
	fmt.Fprintf(&b, "- FPS: %d\n\n", fps)

	if s := strings.TrimSpace(p.BriefDesc); s != "" {
		b.WriteString("## Mô tả video gốc\n" + s + "\n\n")
	}
	if s := strings.TrimSpace(p.EditPrompt); s != "" {
		b.WriteString("## Yêu cầu edit\n" + s + "\n\n")
	}

	writeAssets(&b, assets)
	writeOptions(&b, p)
	writeOutput(&b, p.ID, w, h, version)

	if s := strings.TrimSpace(extra); s != "" {
		b.WriteString("## Dặn dò thêm\n" + s + "\n")
	}
	return b.String()
}

// writeAssets liệt kê asset theo đường dẫn tương đối assets/<file>.
func writeAssets(b *strings.Builder, assets []store.Asset) {
	b.WriteString("## Danh sách asset\n")
	if len(assets) == 0 {
		b.WriteString("(chưa có asset nào — kiểm tra thư mục assets/ nếu cần)\n\n")
		return
	}
	for _, a := range assets {
		name := a.Name
		if a.Path != "" {
			name = filepath.Base(a.Path)
		}
		fmt.Fprintf(b, "- assets/%s — loại: %s", name, a.Kind)
		if a.Duration > 0 {
			fmt.Fprintf(b, ", thời lượng: %.1fs", a.Duration)
		}
		fmt.Fprintf(b, ", thứ tự: %d", a.Order)
		if d := strings.TrimSpace(a.Desc); d != "" {
			fmt.Fprintf(b, " — mô tả: %s", d)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// writeOptions ghi các tùy chọn xử lý được bật của dự án.
func writeOptions(b *strings.Builder, p store.Project) {
	var opts []string
	if p.AutoCut {
		opts = append(opts, "Tự động cắt bỏ khoảng lặng, khoảng đen và đoạn thừa.")
	}
	if p.AutoSub {
		opts = append(opts, "Tạo phụ đề karaoke khớp giọng nói, xuất file .srt vào outputs/.")
	}
	if p.AutoKey {
		opts = append(opts, "Phân tích nội dung, chọn từ khóa chính để làm nổi bật.")
	}
	if len(p.Keywords) > 0 {
		opts = append(opts, "Từ khóa cần chú ý: "+strings.Join(p.Keywords, ", ")+".")
	}
	if len(opts) == 0 {
		return
	}
	b.WriteString("## Tùy chọn xử lý\n")
	for _, o := range opts {
		b.WriteString("- " + o + "\n")
	}
	b.WriteString("\n")
}

// writeOutput ghi yêu cầu output bắt buộc (file mp4 + meta.json + báo cáo).
func writeOutput(b *strings.Builder, projectID string, w, h, version int) {
	out := fmt.Sprintf("outputs/%s-v%d.mp4", projectID, version)
	b.WriteString("## YÊU CẦU OUTPUT (BẮT BUỘC)\n")
	fmt.Fprintf(b, "1. Ghi video kết quả vào %s, đúng kích thước %dx%d.\n", out, w, h)
	fmt.Fprintf(b, "2. Khi hoàn tất, ghi file meta.json vào thư mục dự án hiện tại với nội dung: {\"status\":\"done\",\"output\":\"%s\"}.\n", out)
	b.WriteString("3. Báo cáo ngắn gọn các bước đã làm.\n\n")
}
