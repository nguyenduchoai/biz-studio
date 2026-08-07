package capcut

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/media"
	"bizstudio/internal/recap"
)

// ---------- Xuất phiên "Phim → Kể chuyện" thành dự án CapCut ----------
//
// Bản draft giữ PHIM CẮT GỌN theo cảnh (không có trò đóng băng khung — đó là
// mẹo của bước render; trong trình dựng người dùng tự quyết): track hình là
// các khúc phim nối nhau, track lời đặt file giọng từng cảnh tại mốc cảnh,
// track chữ là lời dẫn. Lời dài hơn cảnh được đẩy sang track lời thứ hai để
// không đè giờ nhau — người dùng xếp lại trong CapCut.

// ExportResult — sản phẩm xuất.
type ExportResult struct {
	DraftDir string `json:"draftDir"` // thư mục draft (tuyệt đối)
	ZipPath  string `json:"zipPath"`  // bản nén để mang đi (tuyệt đối)
	Voiced   int    `json:"voiced"`   // số cảnh có lời kèm giọng
	Overflow int    `json:"overflow"` // số lời phải nằm track phụ
	Clamped  int    `json:"clamped"`  // số lời bị cắt hiển thị vì hết chỗ cả hai track
}

// FromRecap dựng draft từ manifest phiên kể chuyện. Đòi hỏi đã render ít nhất
// một lần (để có file giọng từng cảnh trong audio/).
func FromRecap(ctx context.Context, dataDir string, m *recap.Manifest, srcAbs string) (*ExportResult, error) {
	info, err := media.Probe(srcAbs)
	if err != nil {
		return nil, fmt.Errorf("đọc thông số phim nguồn: %w", err)
	}
	if info.Width <= 0 || info.Height <= 0 {
		return nil, fmt.Errorf("phim nguồn không có thông số khung hình")
	}

	name := "BizStudio-" + m.ID
	d, err := New(name, info.Width, info.Height, 30)
	if err != nil {
		return nil, err
	}

	res := &ExportResult{}
	audioDir := filepath.Join(recap.Dir(dataDir, m.ID), "audio")
	srcDur := Micro(info.Duration)

	// Con trỏ chống đè giờ của hai track lời.
	mainEnd, overEnd := int64(0), int64(0)

	cursor := int64(0)
	for _, sc := range m.Scenes {
		sceneDur := Micro(sc.End - sc.Start)
		if sceneDur <= 0 {
			continue
		}
		if err := d.AddVideo(srcAbs, Micro(sc.Start), sceneDur, cursor, srcDur, info.Width, info.Height); err != nil {
			return nil, fmt.Errorf("cảnh %d: %w", sc.Index, err)
		}

		text := strings.TrimSpace(sc.Text)
		if text != "" {
			wav := filepath.Join(audioDir, fmt.Sprintf("seg-%03d.wav", sc.Index))
			if st, err := os.Stat(wav); err == nil && st.Size() > 0 {
				vInfo, perr := media.Probe(wav)
				if perr == nil && vInfo.Duration > 0 {
					vDur := Micro(vInfo.Duration)
					switch {
					case cursor >= mainEnd:
						if err := d.AddAudio(wav, cursor, vDur, vDur, false); err != nil {
							return nil, fmt.Errorf("lời cảnh %d: %w", sc.Index, err)
						}
						mainEnd = cursor + vDur
					case cursor >= overEnd:
						// lời trước còn tràn sang cảnh này → track phụ
						if err := d.AddAudio(wav, cursor, vDur, vDur, true); err != nil {
							return nil, fmt.Errorf("lời cảnh %d: %w", sc.Index, err)
						}
						overEnd = cursor + vDur
						res.Overflow++
					default:
						// Cả hai track đều bận (lời trước tràn qua hơn một cảnh):
						// dời điểm đặt tới chỗ trống sớm nhất và cắt bớt phần đuôi
						// cho khớp mốc kết thúc gốc — có đếm và báo trong kết quả,
						// không im lặng nuốt lời.
						start, over := mainEnd, false
						if overEnd < mainEnd {
							start, over = overEnd, true
						}
						if dur := cursor + vDur - start; dur > 0 {
							if err := d.AddAudio(wav, start, dur, vDur, over); err != nil {
								return nil, fmt.Errorf("lời cảnh %d: %w", sc.Index, err)
							}
							if over {
								overEnd = start + dur
							} else {
								mainEnd = start + dur
							}
						}
						res.Clamped++
					}
					res.Voiced++
				}
			}
			subDur := sceneDur
			if err := d.AddText(text, cursor, subDur); err != nil {
				return nil, fmt.Errorf("chữ cảnh %d: %w", sc.Index, err)
			}
		}
		cursor += sceneDur
	}

	outDir := filepath.Join(recap.Dir(dataDir, m.ID), "capcut")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("tạo thư mục xuất: %w", err)
	}
	folder, err := d.Save(outDir)
	if err != nil {
		return nil, err
	}
	res.DraftDir = folder

	zipPath := folder + ".zip"
	if err := zipDraft(folder, zipPath); err != nil {
		return nil, err
	}
	res.ZipPath = zipPath
	return res, nil
}

// zipDraft nén thư mục draft, GIỮ tên thư mục làm gốc trong zip — giải nén ra
// là có ngay một thư mục draft đặt thẳng vào kho draft của CapCut.
func zipDraft(dir, zipPath string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("tạo file zip: %w", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	base := filepath.Base(dir)
	return filepath.Walk(dir, func(p string, st os.FileInfo, err error) error {
		if err != nil || st.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(filepath.Join(base, rel)))
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
}
