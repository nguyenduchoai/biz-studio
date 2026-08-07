package media

import "testing"

// Parse mốc cảnh từ stderr thu sẵn — khoá hành vi không phụ thuộc binary ffmpeg.
func TestParseSceneCuts(t *testing.T) {
	se := `
[Parsed_showinfo_1 @ 0x600] n:   0 pts:  90090 pts_time:3.003   pos: 1 fmt:yuv420p
[Parsed_showinfo_1 @ 0x600] n:   1 pts: 180180 pts_time:6.006   pos: 2 fmt:yuv420p
[Parsed_showinfo_1 @ 0x600] n:   2 pts: 180180 pts_time:6.006   pos: 2 fmt:yuv420p
[Parsed_showinfo_1 @ 0x600] n:   3 pts: 270270 pts_time:9.5     pos: 3 fmt:yuv420p
dòng rác không liên quan pts_time:xyz
`
	got := ParseSceneCuts(se)
	want := []float64{3.003, 6.006, 9.5}
	if len(got) != len(want) {
		t.Fatalf("số mốc = %d, cần %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if diff := got[i] - want[i]; diff > 0.0001 || diff < -0.0001 {
			t.Errorf("mốc %d = %v, cần %v", i, got[i], want[i])
		}
	}
}

// Cảnh ngắn hơn ngưỡng phải được GỘP vào cảnh trước — không vứt, không đứng riêng.
func TestScenesFromCutsGopCanhNgan(t *testing.T) {
	// mốc 5.0 và 5.8: cảnh (5.0→5.8) chỉ 0.8s < 2s nên mốc 5.8 bị bỏ,
	// cảnh mở từ 5.0 chạy tới mốc kế đủ dài.
	scenes := scenesFromCuts([]float64{5.0, 5.8, 10.0}, 30.0, 2.0)
	want := [][2]float64{{0, 5}, {5, 10}, {10, 30}}
	if len(scenes) != len(want) {
		t.Fatalf("số cảnh = %d, cần %d: %+v", len(scenes), len(want), scenes)
	}
	for i, w := range want {
		if scenes[i].Start != w[0] || scenes[i].End != w[1] {
			t.Errorf("cảnh %d = [%v→%v], cần [%v→%v]", i, scenes[i].Start, scenes[i].End, w[0], w[1])
		}
	}
}

// Đoạn cuối ngắn phải nhập vào cảnh trước, và tổng các cảnh phải phủ kín video.
func TestScenesFromCutsDoanCuoiNgan(t *testing.T) {
	scenes := scenesFromCuts([]float64{5.0, 9.5}, 10.0, 2.0) // đoạn cuối 0.5s
	if len(scenes) != 2 {
		t.Fatalf("số cảnh = %d, cần 2: %+v", len(scenes), scenes)
	}
	if scenes[1].End != 10.0 {
		t.Errorf("cảnh cuối phải kết thúc ở 10.0, nhận %v", scenes[1].End)
	}
	total := 0.0
	for _, s := range scenes {
		total += s.Duration()
	}
	if total != 10.0 {
		t.Errorf("tổng thời lượng các cảnh = %v, phải phủ kín 10.0", total)
	}
}

// Không có mốc nào → cả video là một cảnh.
func TestScenesFromCutsKhongCoMoc(t *testing.T) {
	scenes := scenesFromCuts(nil, 12.0, 2.0)
	if len(scenes) != 1 || scenes[0].Start != 0 || scenes[0].End != 12.0 {
		t.Fatalf("cần đúng 1 cảnh [0→12], nhận %+v", scenes)
	}
}

// Gộp về tối đa N cảnh: chọn cặp liền kề ngắn nhất, và phải BÁO số lần gộp.
func TestMergeToMaxScenes(t *testing.T) {
	scenes := []Scene{
		{Index: 0, Start: 0, End: 10},
		{Index: 1, Start: 10, End: 12}, // 2s
		{Index: 2, Start: 12, End: 14}, // 2s — cặp 1+2 tổng 4s là nhỏ nhất
		{Index: 3, Start: 14, End: 30},
	}
	out, merged := MergeToMaxScenes(scenes, 3)
	if merged != 1 {
		t.Fatalf("số lần gộp = %d, cần 1", merged)
	}
	if len(out) != 3 {
		t.Fatalf("số cảnh = %d, cần 3: %+v", len(out), out)
	}
	if out[1].Start != 10 || out[1].End != 14 {
		t.Errorf("cảnh gộp phải là [10→14], nhận [%v→%v]", out[1].Start, out[1].End)
	}
	if out[2].Index != 2 {
		t.Errorf("Index phải được đánh lại liên tục, cảnh cuối Index=%d", out[2].Index)
	}
	// không cần gộp thì trả nguyên
	same, m0 := MergeToMaxScenes(out, 10)
	if m0 != 0 || len(same) != 3 {
		t.Errorf("không vượt trần thì không được gộp (merged=%d, len=%d)", m0, len(same))
	}
}
