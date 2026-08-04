package media

import (
	"math"
	"testing"

	"bizstudio/internal/whisper"
)

// words mô phỏng 2 câu: "…thừa." kết thúc 1.36s, câu sau bắt đầu 3.00s.
func guardWords() []whisper.Word {
	return []whisper.Word{
		{Text: "Chúng", Start: 0.00, End: 0.22},
		{Text: "thừa.", Start: 1.12, End: 1.36},
		{Text: "Việc", Start: 3.00, End: 3.24},
		{Text: "nhiều.", Start: 5.40, End: 5.72},
	}
}

// Khoảng lặng ffmpeg dò được thường BẮT ĐẦU SỚM hơn chỗ từ thật sự kết thúc
// (âm cuối c/t/p/ch không hữu thanh) → cắt thô là nuốt chữ.
func TestSafeCutRangesKhongChamVaoTu(t *testing.T) {
	opt := AutoCutOpt{}.withDefaults()
	sil := []TimeSpan{{Start: 1.24, End: 3.08}} // lấn vào "thừa." và cả "Việc"
	cuts := safeCutRanges(sil, guardWords(), opt)

	if len(cuts) != 1 {
		t.Fatalf("muốn 1 đoạn cắt, có %d: %+v", len(cuts), cuts)
	}
	// Phải bắt đầu SAU 1.36+0.18 và kết thúc TRƯỚC 3.00-0.12.
	if cuts[0].Start < 1.54-1e-9 || cuts[0].End > 2.88+1e-9 {
		t.Errorf("đoạn cắt %+v lấn vào vùng bảo vệ [1.54, 2.88]", cuts[0])
	}

	guards := WordGuards(guardWords(), opt.PadStart, opt.PadEnd)
	for _, c := range cuts {
		for _, g := range guards {
			if c.Start < g.End && g.Start < c.End {
				t.Errorf("đoạn cắt %+v chạm vùng bảo vệ %+v", c, g)
			}
		}
	}
}

// Khoảng lặng nằm gọn trong một từ (ngắt hơi giữa chữ) thì KHÔNG được cắt.
func TestSafeCutRangesBoQuaKhoangLangTrongTu(t *testing.T) {
	opt := AutoCutOpt{}.withDefaults()
	sil := []TimeSpan{{Start: 1.15, End: 1.34}, {Start: 3.05, End: 3.20}}
	if cuts := safeCutRanges(sil, guardWords(), opt); len(cuts) != 0 {
		t.Errorf("không được cắt gì, nhưng có %+v", cuts)
	}
}

// Không có transcript: chỉ cắt khoảng lặng dài ≥ 2× MinSilence.
func TestLooseCutRangesChiCatKhoangLangDai(t *testing.T) {
	opt := AutoCutOpt{}.withDefaults() // MinSilence 0.6 → ngưỡng 1.2s
	sil := []TimeSpan{
		{Start: 1.0, End: 1.9},  // 0.9s — bỏ qua
		{Start: 3.0, End: 4.9},  // 1.9s — cắt
		{Start: 6.0, End: 7.25}, // 1.25s — cắt
	}
	cuts := looseCutRanges(sil, opt)
	if len(cuts) != 2 {
		t.Fatalf("muốn 2 đoạn cắt, có %d: %+v", len(cuts), cuts)
	}
	if math.Abs(cuts[0].Start-(3.0+opt.PadEnd)) > 1e-9 || math.Abs(cuts[0].End-(4.9-opt.PadStart)) > 1e-9 {
		t.Errorf("đoạn cắt %+v chưa chừa đệm hai đầu", cuts[0])
	}
}

func TestKeepsFromCuts(t *testing.T) {
	keeps, removed := keepsFromCuts([]TimeSpan{{Start: 1.5, End: 2.9}}, 6.0, 0.25)
	if len(keeps) != 2 {
		t.Fatalf("muốn 2 đoạn giữ, có %+v", keeps)
	}
	if keeps[0] != (segment{0, 1.5}) || keeps[1] != (segment{2.9, 6.0}) {
		t.Errorf("đoạn giữ sai: %+v", keeps)
	}
	if math.Abs(removed-1.4) > 1e-9 {
		t.Errorf("bỏ đi %.3fs, muốn 1.4s", removed)
	}
	// Mẩu vụn đầu file (< minKeep) phải bị bỏ luôn.
	keeps, _ = keepsFromCuts([]TimeSpan{{Start: 0.1, End: 2.0}}, 6.0, 0.25)
	if len(keeps) != 1 || keeps[0] != (segment{2.0, 6.0}) {
		t.Errorf("đoạn giữ sai khi có mẩu vụn đầu file: %+v", keeps)
	}
}

func TestSubtractSpans(t *testing.T) {
	guards := []TimeSpan{{Start: 1.0, End: 2.0}, {Start: 3.0, End: 4.0}}
	got := subtractSpans(TimeSpan{Start: 0.5, End: 5.0}, guards)
	want := []TimeSpan{{0.5, 1.0}, {2.0, 3.0}, {4.0, 5.0}}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("phần %d = %+v, muốn %+v", i, got[i], want[i])
		}
	}
	if n := len(subtractSpans(TimeSpan{Start: 1.2, End: 1.8}, guards)); n != 0 {
		t.Errorf("khoảng nằm gọn trong vùng bảo vệ phải bị bỏ hết, còn %d phần", n)
	}
}
