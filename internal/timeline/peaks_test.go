package timeline

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

// Sóng âm phải phản ánh ĐÚNG chỗ có tiếng. Dựng file nửa đầu im nửa sau kêu rồi
// kiểm: nếu đảo ngược, hoặc dàn đều, thì người dùng cắt theo hình sẽ cắt trượt.
func TestPeaksFollowsTheAudio(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "half.wav")
	// 2 giây im lặng rồi 2 giây tone.
	//
	// volume=8 là cố ý: nguồn sine của ffmpeg mặc định biên độ 0.125 (-18 dB).
	// Đo bằng file mờ như vậy thì ngưỡng nào cũng thành tuỳ tiện.
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono:d=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-filter_complex", "[1:a]volume=8[loud];[0:a][loud]concat=n=2:v=0:a=1[a]",
		"-map", "[a]", p).CombinedOutput()
	if err != nil {
		t.Fatalf("dựng file thử: %v — %s", err, out)
	}

	peaks, err := Peaks(context.Background(), p, 100)
	if err != nil {
		t.Fatalf("Peaks: %v", err)
	}
	if len(peaks) != 100 {
		t.Fatalf("trả %d ô, muốn 100", len(peaks))
	}

	var firstHalf, secondHalf float64
	for i, v := range peaks {
		if v < 0 || v > 1 {
			t.Fatalf("ô %d = %.3f, phải nằm trong 0..1", i, v)
		}
		if i < 50 {
			firstHalf += v
		} else {
			secondHalf += v
		}
	}
	t.Logf("nửa đầu (im lặng) trung bình %.3f · nửa sau (có tiếng) trung bình %.3f",
		firstHalf/50, secondHalf/50)
	if firstHalf > 1 {
		t.Errorf("nửa im lặng lại có sóng (tổng %.2f) — vẽ ra sẽ đánh lừa người cắt", firstHalf)
	}
	// So TỈ LỆ chứ không so mức tuyệt đối: mức phụ thuộc file, còn thứ phải đúng
	// là chỗ nào có tiếng thì sóng phải cao hơn hẳn chỗ im.
	if secondHalf < firstHalf*10+10 {
		t.Errorf("nửa có tiếng (%.2f) không nổi hơn hẳn nửa im (%.2f) — không thấy gì để mà cắt",
			secondHalf, firstHalf)
	}
	if avg := secondHalf / 50; avg < 0.5 {
		t.Errorf("đỉnh trung bình %.3f, quá thấp so với tone đã khuếch đại — nghi đọc mẫu sai", avg)
	}
}

// Xin nhiều ô hơn số mẫu thì phải tự thu về, không trả mảng có ô rỗng.
func TestPeaksCapsBucketsToSampleCount(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	p := gen(t, dir, "tiny.wav", "sine=frequency=440:duration=0.01")
	peaks, err := Peaks(context.Background(), p, 4000)
	if err != nil {
		t.Fatalf("Peaks: %v", err)
	}
	if len(peaks) > 4000 || len(peaks) == 0 {
		t.Fatalf("trả %d ô cho file 0.01 giây", len(peaks))
	}
	t.Logf("file 0.01s → %d ô", len(peaks))
}

func TestPeaksErrorsOnNonAudio(t *testing.T) {
	requireFFmpeg(t)
	if _, err := Peaks(context.Background(), filepath.Join(t.TempDir(), "khong-co.wav"), 100); err == nil {
		t.Error("file không tồn tại mà không báo lỗi")
	}
}

// -32768 không có số đối trong int16; đảo dấu thẳng là tràn về chính nó (số âm)
// và phép so sánh đỉnh hỏng lặng lẽ.
func TestAbs16HandlesMinInt16(t *testing.T) {
	if got := abs16(-32768); got <= 0 {
		t.Errorf("abs16(-32768) = %d, phải là số dương", got)
	}
	if got := abs16(-5); got != 5 {
		t.Errorf("abs16(-5) = %d", got)
	}
}
