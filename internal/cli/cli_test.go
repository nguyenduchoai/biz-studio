package cli

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Gói flag của Go dừng phân tích ở đối số đầu tiên KHÔNG phải cờ. Nghĩa là
// `normalize video.mp4 --platform tiktok` bỏ qua hẳn --platform rồi báo một lỗi
// chẳng liên quan — mà đó lại là cách viết tự nhiên nhất, cả người lẫn agent
// đều gõ thế. Đây là lỗi đã gặp thật khi thử tay.
func TestParseKhongPhuThuocThuTu(t *testing.T) {
	build := func() (*flag.FlagSet, *string, *bool, *int) {
		f := fs("thu")
		s := f.String("platform", "", "")
		b := f.Bool("dry-run", false, "")
		n := f.Int("fps", 0, "")
		return f, s, b, n
	}
	cases := []struct {
		ten  string
		args []string
	}{
		{"cờ đứng trước", []string{"--platform", "tiktok", "--dry-run", "--fps", "30", "phim.mp4"}},
		{"cờ đứng sau", []string{"phim.mp4", "--platform", "tiktok", "--dry-run", "--fps", "30"}},
		{"cờ xen kẽ", []string{"--platform", "tiktok", "phim.mp4", "--dry-run", "--fps", "30"}},
		{"dạng ten=gia-tri", []string{"phim.mp4", "--platform=tiktok", "--dry-run", "--fps=30"}},
		{"một gạch", []string{"phim.mp4", "-platform", "tiktok", "-dry-run", "-fps", "30"}},
	}
	for _, c := range cases {
		f, plat, dry, fps := build()
		if err := parse(f, c.args); err != nil {
			t.Errorf("%s: %v", c.ten, err)
			continue
		}
		if *plat != "tiktok" || !*dry || *fps != 30 {
			t.Errorf("%s: platform=%q dry=%v fps=%d — muốn tiktok/true/30", c.ten, *plat, *dry, *fps)
		}
		if f.NArg() != 1 || f.Arg(0) != "phim.mp4" {
			t.Errorf("%s: đối số thường = %v, muốn [phim.mp4]", c.ten, f.Args())
		}
	}
}

// Cờ luận lý KHÔNG được nuốt đối số đứng sau nó, nếu không tên file bị ăn mất.
func TestParseCoLuanLyKhongNuotDoiSo(t *testing.T) {
	f := fs("thu")
	dry := f.Bool("dry-run", false, "")
	if err := parse(f, []string{"--dry-run", "phim.mp4"}); err != nil {
		t.Fatal(err)
	}
	if !*dry {
		t.Error("--dry-run không được nhận")
	}
	if f.NArg() != 1 || f.Arg(0) != "phim.mp4" {
		t.Errorf("đối số thường = %v — cờ luận lý đã nuốt mất tên file", f.Args())
	}
}

// Sau "--" mọi thứ là đối số thường, kể cả khi trông như cờ. Cần cho tên file
// bắt đầu bằng dấu gạch.
func TestParseDauNganCach(t *testing.T) {
	f := fs("thu")
	p := f.String("platform", "", "")
	if err := parse(f, []string{"--platform", "tiktok", "--", "-ten-la.mp4"}); err != nil {
		t.Fatal(err)
	}
	if *p != "tiktok" {
		t.Errorf("platform = %q", *p)
	}
	if f.NArg() != 1 || f.Arg(0) != "-ten-la.mp4" {
		t.Errorf("đối số thường = %v, muốn [-ten-la.mp4]", f.Args())
	}
}

// Thiếu công cụ ngoài phải ra loại "dependency" — agent biết đi cài, chứ không
// phải thử lại mù. Lỗi chung chung thì nó chỉ biết thử lại.
func TestNeedToolsRaLoaiDependency(t *testing.T) {
	err := needTools("cong-cu-chac-chan-khong-ton-tai-xyz123")
	if err == nil {
		t.Fatal("công cụ bịa mà vẫn cho qua")
	}
	if k := KindOf(err); k != KindDependency {
		t.Errorf("loại lỗi = %q, muốn %q", k, KindDependency)
	}
	if !strings.Contains(err.Error(), "cong-cu-chac-chan-khong-ton-tai-xyz123") {
		t.Errorf("thông điệp không nêu tên công cụ thiếu: %s", err)
	}
	// công cụ có thật thì phải qua
	if err := needTools("sh"); err != nil {
		t.Errorf("sh có thật mà báo thiếu: %v", err)
	}
}

// Mã thoát phải khác nhau theo loại lỗi, để script dùng được mà không phải đọc
// JSON.
func TestMaThoatTheoLoaiLoi(t *testing.T) {
	cases := map[Kind]int{
		KindUsage: 2, KindDependency: 3, KindRetryable: 4, KindFailed: 1,
	}
	for k, muon := range cases {
		if got := exitCode(&errBody{Kind: k}); got != muon {
			t.Errorf("loại %q → mã %d, muốn %d", k, got, muon)
		}
	}
	if got := exitCode(nil); got != 1 {
		t.Errorf("không có lỗi cụ thể → mã %d, muốn 1", got)
	}
}

// Manifest là cách nối lệnh: lệnh sau chỉ cần trỏ đúng thư mục là dùng lại được
// kết quả lệnh trước.
func TestManifestNoiLenh(t *testing.T) {
	dir := t.TempDir()
	Now = func() string { return "2026-01-01T00:00:00Z" }

	// thư mục trống phải trả manifest RỖNG chứ không lỗi
	m := LoadManifest(dir)
	if m == nil || len(m.Stages) != 0 {
		t.Fatalf("thư mục trống phải cho manifest rỗng, nhận %+v", m)
	}
	if m.Get("voice") != "" {
		t.Error("manifest rỗng mà trả ra đường dẫn")
	}

	if err := m.Save(dir, "tts", ManifestStage{
		Command: "tts", At: Now(), Outputs: map[string]string{"voice": "/x/voice.wav"},
	}); err != nil {
		t.Fatal(err)
	}
	// lệnh sau đọc lại
	m2 := LoadManifest(dir)
	if got := m2.Get("voice"); got != "/x/voice.wav" {
		t.Errorf("đọc lại voice = %q", got)
	}
	// lệnh thứ hai không được xoá kết quả lệnh đầu
	if err := m2.Save(dir, "broll", ManifestStage{
		Command: "broll", At: Now(), Outputs: map[string]string{"video": "/x/ra.mp4"},
	}); err != nil {
		t.Fatal(err)
	}
	m3 := LoadManifest(dir)
	if m3.Get("voice") != "/x/voice.wav" || m3.Get("video") != "/x/ra.mp4" {
		t.Errorf("lệnh sau làm mất kết quả lệnh trước: %+v", m3.Outputs)
	}
	if len(m3.Stages) != 2 {
		t.Errorf("có %d chặng, muốn 2", len(m3.Stages))
	}
}

// Manifest hỏng không được chặn người dùng chạy tiếp — mất một file trạng thái
// thì làm lại, chứ không phải bó tay.
func TestManifestHongVanChayTiep(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, manifestName), []byte("{{{ không phải JSON"), 0o644)
	m := LoadManifest(dir)
	if m == nil || m.Stages == nil || m.Outputs == nil {
		t.Fatalf("manifest hỏng phải cho bản rỗng dùng được, nhận %+v", m)
	}
}

// Kết quả phải là JSON hợp lệ và đọc lại được — đây là toàn bộ giao ước với
// agent.
func TestResultLaJSONHopLe(t *testing.T) {
	for _, r := range []Result{
		{OK: true, Command: "probe", Stats: map[string]any{"durationSec": 8.5}},
		Fail("normalize", Usage("nền tảng %q không có", "myspace")),
		Fail("probe", Dependency("thiếu ffprobe")),
	} {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("%+v: %v", r, err)
		}
		if strings.Contains(string(raw), "\n") {
			t.Errorf("kết quả có xuống dòng — phải gói gọn MỘT dòng: %s", raw)
		}
		var back map[string]any
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Errorf("không đọc lại được: %v", err)
		}
		if back["ok"] != r.OK {
			t.Errorf("trường ok sai: %v", back["ok"])
		}
	}
}

// Bản .app trên macOS gọi thẳng `bizstudio -port 6868 -data …`. Đổi cách điều
// phối mà quên ca này là app trên máy người dùng không mở nổi.
func TestIsServeModeGiuDuocDuongCu(t *testing.T) {
	for _, c := range []struct {
		args []string
		muon bool
	}{
		{nil, true},
		{[]string{}, true},
		{[]string{"-port", "6868", "-data", "x"}, true},
		{[]string{"--port=6868"}, true},
		{[]string{"probe", "a.mp4"}, false},
		{[]string{"broll", "--clips", "x"}, false},
		{[]string{"help"}, false},
	} {
		if got := IsServeMode(c.args); got != c.muon {
			t.Errorf("IsServeMode(%v) = %v, muốn %v", c.args, got, c.muon)
		}
	}
}

// Mọi lệnh trong bảng phải có đủ tên, mô tả và hàm chạy — thiếu cái nào thì
// trợ giúp hiện dòng trống hoặc gọi vào nil.
func TestBangLenhToanVen(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range All() {
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Short) == "" || c.Run == nil {
			t.Errorf("lệnh thiếu trường: %+v", c)
		}
		if seen[c.Name] {
			t.Errorf("trùng tên lệnh: %q", c.Name)
		}
		seen[c.Name] = true
	}
	if len(All()) < 5 {
		t.Errorf("chỉ có %d lệnh", len(All()))
	}
}
