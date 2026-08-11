package highlight

import (
	"testing"

	"bizstudio/internal/media"
	"bizstudio/internal/whisper"
)

func tr(segs ...whisper.Segment) *whisper.Transcript {
	return &whisper.Transcript{Segments: segs}
}

func seg(s, e float64, txt string, words ...whisper.Word) whisper.Segment {
	return whisper.Segment{Start: s, End: e, Text: txt, Words: words}
}

func TestCandidatesGopCauNgan(t *testing.T) {
	got := Candidates(tr(
		seg(0, 0.6, "Chào"), // quá ngắn, gộp với câu sau
		seg(0.7, 3.0, "mọi người hôm nay tôi nói về giá điện"),
		seg(3.2, 8.0, "Hoá đơn tháng này tăng bốn mươi phần trăm"),
		seg(8.1, 8.3, ""), // rỗng, bỏ
	))
	if len(got) != 2 {
		for i, c := range got {
			t.Logf("  [%d] %.1f-%.1f %q", i, c.Start, c.End, c.Text)
		}
		t.Fatalf("muốn 2 đoạn, nhận %d", len(got))
	}
	if got[0].Start != 0 || got[0].End != 3.0 {
		t.Errorf("đoạn đầu %.1f-%.1f, muốn 0.0-3.0 (câu 'Chào' phải được gộp vào)", got[0].Start, got[0].End)
	}
	if got[0].Index != 0 || got[1].Index != 1 {
		t.Errorf("số thứ tự sai: %d, %d", got[0].Index, got[1].Index)
	}
}

func TestCandidatesChanDoanQuaDai(t *testing.T) {
	got := Candidates(tr(seg(0, 60, "một đoạn rất dài")))
	if len(got) != 1 {
		t.Fatalf("muốn 1 đoạn, nhận %d", len(got))
	}
	if d := got[0].Dur(); d != maxCandidate {
		t.Errorf("đoạn dài %.1fs, phải bị chặn ở %.1fs — không chặn thì một đoạn nuốt hết chỗ của clip", d, maxCandidate)
	}
}

// Chọn theo ĐIỂM nhưng xếp theo THỜI GIAN. Ghép theo thứ tự điểm thì câu chuyện
// nhảy cóc tới lui, người xem không lần ra mạch.
func TestPickChonTheoDiemXepTheoThoiGian(t *testing.T) {
	cs := []Candidate{
		{Index: 0, Start: 0, End: 5, Score: 3},    // điểm thấp, bỏ
		{Index: 1, Start: 10, End: 15, Score: 9},  // cao
		{Index: 2, Start: 20, End: 25, Score: 7},  // vừa
		{Index: 3, Start: 30, End: 35, Score: 10}, // cao nhất
	}
	got := Pick(cs, 15, 5)
	if len(got) != 3 {
		t.Fatalf("muốn giữ 3 đoạn (15 giây), nhận %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Start < got[i-1].Start {
			t.Errorf("kết quả không theo thứ tự thời gian: đoạn %d bắt đầu %.1f, sau đoạn bắt đầu %.1f",
				i, got[i].Start, got[i-1].Start)
		}
	}
	for _, c := range got {
		if c.Score < 5 {
			t.Errorf("đoạn điểm %.0f lọt qua ngưỡng 5", c.Score)
		}
	}
}

func TestPickKhongVuotThoiLuong(t *testing.T) {
	cs := []Candidate{
		{Index: 0, Start: 0, End: 8, Score: 9},
		{Index: 1, Start: 10, End: 18, Score: 8},
		{Index: 2, Start: 20, End: 22, Score: 7}, // 2 giây, vừa chỗ còn lại
	}
	got := Pick(cs, 10, 5)
	if d := TotalDur(got); d > 10 {
		t.Errorf("tổng %.1fs, vượt đích 10s", d)
	}
	// Đoạn 8 giây điểm cao vào trước, còn 2 giây — phải lấy đoạn 2 giây điểm
	// thấp hơn chứ không bỏ trống chỗ.
	if len(got) != 2 {
		t.Errorf("muốn 2 đoạn (8s + 2s), nhận %d", len(got))
	}
	if Pick(cs, 0, 5) != nil {
		t.Error("thời lượng đích 0 phải trả rỗng")
	}
}

// Mép cắt phải nới ra tới TỪ trọn vẹn rồi mới đệm. Cắt đúng mốc câu vẫn xén mất
// nửa âm — mốc câu của bản bóc băng không khớp chính xác mốc từ.
func TestSnapToWordsBaoTronTu(t *testing.T) {
	x := tr(seg(1.0, 3.0, "xin chào các bạn",
		whisper.Word{Text: "xin", Start: 0.85, End: 1.2}, // bắt đầu TRƯỚC mốc câu
		whisper.Word{Text: "chào", Start: 1.3, End: 1.8},
		whisper.Word{Text: "các", Start: 2.0, End: 2.4},
		whisper.Word{Text: "bạn", Start: 2.6, End: 3.25})) // kết thúc SAU mốc câu
	got := snapToWords([]Candidate{{Start: 1.0, End: 3.0}}, x, 100)
	if len(got) != 1 {
		t.Fatalf("muốn 1 khoảng, nhận %d", len(got))
	}
	wantS, wantE := 0.85-padStart, 3.25+padEnd
	if d := got[0].Start - wantS; d > 0.001 || d < -0.001 {
		t.Errorf("mép đầu %.3f, muốn %.3f — chưa bao trọn từ 'xin'", got[0].Start, wantS)
	}
	if d := got[0].End - wantE; d > 0.001 || d < -0.001 {
		t.Errorf("mép cuối %.3f, muốn %.3f — chưa bao trọn từ 'bạn'", got[0].End, wantE)
	}
}

func TestSnapToWordsKhongVuotFile(t *testing.T) {
	x := tr(seg(0, 5, "a", whisper.Word{Text: "a", Start: 0, End: 5}))
	got := snapToWords([]Candidate{{Start: 0, End: 5}}, x, 5)
	if got[0].Start < 0 {
		t.Errorf("mép đầu âm: %.3f", got[0].Start)
	}
	if got[0].End > 5 {
		t.Errorf("mép cuối %.3f vượt quá thời lượng file 5s", got[0].End)
	}
}

func TestMergeClose(t *testing.T) {
	got := mergeClose([]media.TimeSpan{{Start: 0, End: 2}, {Start: 2.3, End: 4}, {Start: 10, End: 12}})
	if len(got) != 2 {
		t.Fatalf("muốn 2 khoảng sau khi nối, nhận %d", len(got))
	}
	if got[0].Start != 0 || got[0].End != 4 {
		t.Errorf("khoảng đầu %.1f-%.1f, muốn 0-4 (cách 0.3s phải nối)", got[0].Start, got[0].End)
	}
	if mergeClose(nil) != nil {
		t.Error("đầu vào rỗng phải trả nil")
	}
}

// Trần nền tảng là ranh giới cứng — quá một giây là video bị xếp sang loại khác.
func TestCapTotal(t *testing.T) {
	in := []media.TimeSpan{{Start: 0, End: 20}, {Start: 30, End: 50}, {Start: 60, End: 70}}
	got, cut := capTotal(in, 30)
	if !cut {
		t.Error("phải báo là đã cắt bớt")
	}
	var total float64
	for _, s := range got {
		total += s.End - s.Start
	}
	if total > 30.001 {
		t.Errorf("tổng %.1fs vượt trần 30s", total)
	}
	// đoạn cuối bị cắt cụt chứ không bỏ hẳn — bỏ hẳn thì mất luôn phần đầu vốn vừa chỗ
	if len(got) != 2 {
		t.Errorf("muốn 2 khoảng (20s + 10s cắt cụt), nhận %d", len(got))
	}
	if _, cut := capTotal(in, 100); cut {
		t.Error("trần rộng hơn tổng thì không được báo đã cắt")
	}
}

func TestParseScores(t *testing.T) {
	cases := map[string]string{
		"JSON trần":      `[{"i":0,"diem":8,"vi":"mở mạnh"},{"i":1,"diem":3,"vi":"câu nối"}]`,
		"bọc khối mã":    "```json\n[{\"i\":0,\"diem\":8},{\"i\":1,\"diem\":3}]\n```",
		"khoá tiếng Anh": `[{"index":0,"score":8},{"index":1,"score":3}]`,
		"điểm là chuỗi":  `[{"i":0,"diem":"8"},{"i":1,"diem":"3"}]`,
		"lẫn chữ thừa":   `Đây là kết quả: [{"i":0,"diem":8},{"i":1,"diem":3}] xong.`,
	}
	for ten, raw := range cases {
		got := parseScores(raw)
		if len(got) != 2 || got[0].Score != 8 || got[1].Score != 3 {
			t.Errorf("%s: đọc ra %+v", ten, got)
		}
	}
	// điểm ngoài thang phải bị kẹp về 0..10
	got := parseScores(`[{"i":0,"diem":99},{"i":1,"diem":-5}]`)
	if got[0].Score != 10 || got[1].Score != 0 {
		t.Errorf("không kẹp điểm ngoài thang: %+v", got)
	}
	if parseScores("không phải JSON") != nil {
		t.Error("chuỗi rác phải trả nil chứ không phải map rỗng")
	}
}
