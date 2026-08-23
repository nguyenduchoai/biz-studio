package qc

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"bizstudio/internal/media"
	"bizstudio/internal/util"
)

// Kích thước lưới mặc định và bề rộng mỗi ô.
const (
	sheetCols  = 5
	sheetRows  = 4
	sheetTileW = 320
)

// ContactSheet dựng một ảnh PNG gồm lưới thumbnail trải đều suốt video.
//
// Vì sao cần: báo cáo QC bằng số bắt được khung đen, khung đứng, tiếng quá nhỏ.
// Nó KHÔNG bắt được những thứ chỉ mắt mới thấy — cảnh lặp, ghép nhầm thứ tự,
// chữ đè lên mặt người, một đoạn lọt vào từ video khác. Muốn thấy thì phải tua
// cả video, mà sau mỗi lần render thì không ai tua.
//
// Một ảnh nhìn hết trong ba giây thay cho việc tua hai mươi phút.
func ContactSheet(ctx context.Context, videoPath, dstPNG string) error {
	info, err := media.Probe(videoPath)
	if err != nil {
		return fmt.Errorf("đọc video để dựng ảnh lưới: %w", err)
	}
	if info.Duration <= 0 {
		return fmt.Errorf("video không có thời lượng hợp lệ: %s", videoPath)
	}
	if err := os.MkdirAll(filepath.Dir(dstPNG), 0o755); err != nil {
		return fmt.Errorf("tạo thư mục cho ảnh lưới: %w", err)
	}

	n := sheetCols * sheetRows
	// Lấy mẫu trải đều suốt video: fps = số ô / thời lượng.
	//
	// Cộng một nhúm vào thời lượng để khung cuối không rơi đúng vào mép file —
	// nhiều video kết bằng một khung đen, mà một ô đen ở góc lưới trông y hệt
	// lỗi thật và làm người xem hoảng vô cớ.
	fps := float64(n) / (info.Duration * 1.02)
	if fps <= 0 || math.IsInf(fps, 0) {
		return fmt.Errorf("không tính được nhịp lấy mẫu cho video %.1fs", info.Duration)
	}

	vf := fmt.Sprintf("fps=%.6f,scale=%d:-1,tile=%dx%d", fps, sheetTileW, sheetCols, sheetRows)
	_, err = util.Run(ctx, "ffmpeg", "-y", "-hide_banner", "-i", videoPath,
		"-vf", vf, "-frames:v", "1", "-update", "1", dstPNG)
	if err != nil {
		return fmt.Errorf("dựng ảnh lưới thất bại: %w", err)
	}
	if fi, serr := os.Stat(dstPNG); serr != nil || fi.Size() == 0 {
		return fmt.Errorf("ffmpeg chạy xong nhưng không sinh ra ảnh lưới: %s", dstPNG)
	}
	return nil
}
