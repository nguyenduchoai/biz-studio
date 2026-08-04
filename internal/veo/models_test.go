package veo

import "testing"

// Chi phí là thứ người dùng nhìn thấy TRƯỚC khi bấm và bị trừ tiền SAU khi bấm.
// Sai ở đây là sai tiền thật, nên các mốc giá được khoá cứng bằng bài kiểm tra.
func TestEstimateUSD(t *testing.T) {
	cases := []struct {
		model, res string
		secs       int
		want       float64
	}{
		{"veo-3.1-generate-preview", "720p", 8, 3.20},
		{"veo-3.1-generate-preview", "1080p", 8, 3.20},
		{"veo-3.1-generate-preview", "4k", 8, 4.80},
		{"veo-3.1-fast-generate-preview", "720p", 8, 0.80},
		{"veo-3.1-fast-generate-preview", "1080p", 8, 0.96},
		{"veo-3.1-lite-generate-preview", "720p", 8, 0.40},
		{"veo-3.1-fast-generate-preview", "720p", 4, 0.40},
	}
	for _, c := range cases {
		got, err := EstimateUSD(c.model, c.res, c.secs, 1)
		if err != nil {
			t.Fatalf("%s %s %ds: %v", c.model, c.res, c.secs, err)
		}
		if diff := got - c.want; diff > 0.001 || diff < -0.001 {
			t.Errorf("%s %s %ds = $%.4f, cần $%.2f", c.model, c.res, c.secs, got, c.want)
		}
	}
}

// Nhiều clip thì nhân lên — đây là chỗ dễ quên nhất khi làm hàng loạt.
func TestEstimateUSDNhieuClip(t *testing.T) {
	got, err := EstimateUSD("veo-3.1-fast-generate-preview", "720p", 8, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != 4.0 {
		t.Errorf("5 clip 8 giây bản nhanh = $%.2f, cần $4.00", got)
	}
}

// Model lite không có 4k: phải BÁO LỖI chứ không được trả $0 — trả 0 thì giao
// diện hiện "miễn phí" trong khi thực tế API sẽ từ chối hoặc tính giá khác.
func TestEstimateUSDDoPhanGiaiKhongHoTro(t *testing.T) {
	if _, err := EstimateUSD("veo-3.1-lite-generate-preview", "4k", 8, 1); err == nil {
		t.Error("lite + 4k phải báo lỗi, không được im lặng trả 0")
	}
	if _, err := EstimateUSD("model-khong-co-that", "720p", 8, 1); err == nil {
		t.Error("model lạ phải báo lỗi")
	}
}

// Veo chỉ nhận 4/6/8 giây. Số lạ phải được kéo về mốc gần nhất TRƯỚC khi gửi,
// không để người dùng chờ vài phút rồi mới nhận lỗi.
func TestNormalizeDuration(t *testing.T) {
	cases := map[int]int{0: 8, 1: 4, 3: 4, 4: 4, 5: 4, 6: 6, 7: 6, 8: 8, 20: 8}
	for in, want := range cases {
		if got := normalizeDuration(in); got != want {
			t.Errorf("normalizeDuration(%d) = %d, cần %d", in, got, want)
		}
	}
}

// 1080p và 4k chỉ tạo được clip 8 giây — chặn trước khi gọi API.
func TestCheckCombo(t *testing.T) {
	if err := checkCombo("1080p", 4); err == nil {
		t.Error("1080p + 4 giây phải bị chặn")
	}
	if err := checkCombo("4k", 6); err == nil {
		t.Error("4k + 6 giây phải bị chặn")
	}
	if err := checkCombo("1080p", 8); err != nil {
		t.Errorf("1080p + 8 giây phải hợp lệ, nhận: %v", err)
	}
	if err := checkCombo("720p", 4); err != nil {
		t.Errorf("720p + 4 giây phải hợp lệ, nhận: %v", err)
	}
}

func TestNormalizeAspectVaResolution(t *testing.T) {
	if normalizeAspect("16:9") != "16:9" || normalizeAspect("") != "9:16" || normalizeAspect("vuông") != "9:16" {
		t.Error("chuẩn hoá khung hình sai")
	}
	if normalizeResolution("1080P") != "1080p" || normalizeResolution("") != "720p" || normalizeResolution("8k") != "720p" {
		t.Error("chuẩn hoá độ phân giải sai")
	}
}
