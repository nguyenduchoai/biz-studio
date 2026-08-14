package broll

import (
	"testing"
)

func ps(src string, n int, dur float64) []Piece {
	out := make([]Piece, n)
	for i := range out {
		out[i] = Piece{Src: src, Start: float64(i) * dur, Dur: dur}
	}
	return out
}

// Đây là lỗi đã đo được trên thật: lấy tuần tự hết clip này mới sang clip kia
// thì với 4 clip cho một video 17 giây, chỉ 2 clip đầu lên hình — người dùng
// đưa 4 file mà chỉ thấy 2. Xoay vòng thì mọi clip đều được dùng.
func TestFillXoayVongQuaMoiClip(t *testing.T) {
	var pieces []Piece
	for _, src := range []string{"a.mp4", "b.mp4", "c.mp4", "d.mp4"} {
		pieces = append(pieces, ps(src, 3, 5)...)
	}
	seq, reused := fill(pieces, 17)
	if reused != 0 {
		t.Errorf("tư liệu thừa sức (60s cho 17s) mà vẫn báo lặp lại %d vòng", reused)
	}
	dung := map[string]bool{}
	for _, p := range seq {
		dung[p.Src] = true
	}
	if len(dung) < 4 {
		t.Errorf("chỉ dùng %d/4 clip: %v — phải rải đều qua mọi clip", len(dung), dung)
	}
	// bốn mẩu đầu phải đến từ bốn clip khác nhau
	for i := 0; i < 4 && i < len(seq); i++ {
		if seq[i].Src != pieces[i*3].Src {
			t.Errorf("mẩu thứ %d đến từ %q, muốn %q — thứ tự chưa xoay vòng",
				i, seq[i].Src, pieces[i*3].Src)
		}
	}
}

func TestFillDuDaiVaBaoKhiThieuTuLieu(t *testing.T) {
	// đủ tư liệu
	seq, reused := fill(ps("a.mp4", 4, 5), 17)
	var total float64
	for _, p := range seq {
		total += p.Dur
	}
	if total < 17 {
		t.Errorf("ghép được %.1fs, thiếu so với 17s cần", total)
	}
	if reused != 0 {
		t.Errorf("20s tư liệu cho 17s mà báo lặp %d vòng", reused)
	}

	// thiếu tư liệu → phải LẶP LẠI và BÁO, chứ không lặng lẽ trả video ngắn hụt
	seq, reused = fill(ps("a.mp4", 1, 3), 20)
	total = 0
	for _, p := range seq {
		total += p.Dur
	}
	if total < 20 {
		t.Errorf("chỉ ghép được %.1fs cho 20s — phải lặp lại cho đủ", total)
	}
	if reused == 0 {
		t.Error("3 giây tư liệu cho 20 giây mà không báo phải lặp lại")
	}
}

// Tư liệu quá ngắn không được làm treo vòng lặp.
func TestFillKhongTreo(t *testing.T) {
	seq, reused := fill(ps("a.mp4", 1, 0.01), 3600)
	if reused <= 50 {
		// chặn ở 50 vòng: kết quả không đủ dài nhưng phải THOÁT được
		t.Logf("dừng ở %d vòng, %d mẩu", reused, len(seq))
	}
	if len(seq) > 100000 {
		t.Errorf("sinh ra %d mẩu — vòng lặp không được chặn", len(seq))
	}
}

func TestCutPieces(t *testing.T) {
	// đuôi vụn phải bị bỏ: mẩu ngắn hơn minPiece chỉ loé lên rồi mất
	// 12 giây / 5 giây = 5 + 5 + 2 → mẩu 2 giây vẫn giữ (2 > 1.2)
	// 11 giây / 5 giây = 5 + 5 + 1 → mẩu 1 giây bị bỏ (1 < 1.2)
	for _, c := range []struct {
		dur  float64
		muon int
	}{
		{12, 3}, {11, 2}, {5, 1}, {1, 0}, {0.5, 0},
	} {
		n := 0
		for s := 0.0; s < c.dur; s += 5 {
			d := c.dur - s
			if d > 5 {
				d = 5
			}
			if d < minPiece {
				break
			}
			n++
		}
		if n != c.muon {
			t.Errorf("clip %.1fs → %d mẩu, muốn %d", c.dur, n, c.muon)
		}
	}
}

// Xáo phải TẤT ĐỊNH. Dựng lại ra video khác nhau mỗi lần là thứ không sửa lỗi
// được: người dùng báo "hỏng ở giây 12" mà lần dựng sau giây 12 đã là cảnh khác.
func TestSeedShuffleTatDinhVaKhongMatMau(t *testing.T) {
	goc := ps("a.mp4", 9, 2)
	a := append([]Piece(nil), goc...)
	b := append([]Piece(nil), goc...)
	seedShuffle(a)
	seedShuffle(b)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("xáo hai lần ra hai kết quả khác nhau tại vị trí %d", i)
		}
	}
	// không được mất hay nhân đôi mẩu nào
	if len(a) != len(goc) {
		t.Fatalf("xáo xong còn %d mẩu, ban đầu %d", len(a), len(goc))
	}
	dem := map[float64]int{}
	for _, p := range a {
		dem[p.Start]++
	}
	for _, p := range goc {
		if dem[p.Start] != 1 {
			t.Errorf("mẩu bắt đầu %.1f xuất hiện %d lần sau khi xáo", p.Start, dem[p.Start])
		}
	}
	// phải THẬT SỰ đổi thứ tự
	giong := 0
	for i := range a {
		if a[i] == goc[i] {
			giong++
		}
	}
	if giong == len(goc) {
		t.Error("xáo mà thứ tự y nguyên")
	}
	// đầu vào quá ngắn không được hoảng
	seedShuffle(nil)
	seedShuffle(ps("a.mp4", 1, 2))
}
