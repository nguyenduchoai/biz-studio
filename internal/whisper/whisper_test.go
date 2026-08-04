package whisper

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"bizstudio/internal/store"
)

// sampleTranscript — 1 câu tiếng Việt có mốc từng từ (mô phỏng faster-whisper).
func sampleTranscript() *Transcript {
	return &Transcript{
		Language: "vi",
		Duration: 3.2,
		Segments: []Segment{{
			Index: 0, Start: 0.5, End: 2.4,
			Text: "Chúng tôi cắt bớt {đoạn} thừa",
			Words: []Word{
				{Text: "Chúng", Start: 0.5, End: 0.9},
				{Text: "tôi", Start: 0.9, End: 1.2},
				{Text: "cắt", Start: 1.5, End: 1.8}, // có khoảng nghỉ 0.3s trước từ này
				{Text: "bớt", Start: 1.8, End: 2.0},
				{Text: "{đoạn}", Start: 2.0, End: 2.2},
				{Text: "thừa", Start: 2.2, End: 2.4},
			},
		}},
	}
}

func TestSRT(t *testing.T) {
	got := SRT(sampleTranscript())
	want := "1\n00:00:00,500 --> 00:00:02,400\nChúng tôi cắt bớt {đoạn} thừa\n\n"
	if got != want {
		t.Errorf("SRT sai:\n%q\nmuốn:\n%q", got, want)
	}
}

func TestKaraokeASSTiming(t *testing.T) {
	ass := KaraokeASS(sampleTranscript(), KaraokeStyle{
		FontName: "Arial Unicode MS", FontSize: 60,
		Primary: "#FFFFFF", Highlight: "#F59E0B", Outline: "#000000",
		PlayResX: 1080, PlayResY: 1920,
	})

	// \k tính bằng centisecond: "Chúng" 0.5→0.9 = 40cs, khoảng nghỉ 0.3s = 30cs.
	for _, want := range []string{
		`{\k40}Chúng `, `{\k30}`, `{\k30}cắt `, `{\k20}thừa`,
		"Dialogue: 0,0:00:00.50,0:00:02.40,Karaoke,,0,0,0,,",
		"PlayResX: 1080", "PlayResY: 1920",
	} {
		if !strings.Contains(ass, want) {
			t.Errorf("thiếu %q trong ASS:\n%s", want, ass)
		}
	}
	// Dấu tiếng Việt phải giữ nguyên, ngoặc nhọn trong nội dung phải được thoát.
	if !strings.Contains(ass, `\{đoạn\}`) {
		t.Errorf("chưa thoát ngoặc nhọn của nội dung:\n%s", ass)
	}
	if strings.Contains(ass, "Chung toi") {
		t.Error("mất dấu tiếng Việt trong ASS")
	}
}

func TestAssColor(t *testing.T) {
	cases := map[string]string{
		"#F59E0B": "&H000B9EF5", // BGR đảo ngược
		"#FFFFFF": "&H00FFFFFF",
		"abc":     "&H00CCBBAA",
		"":        "&H00FFFFFF",
	}
	for in, want := range cases {
		if got := assColor(in); got != want {
			t.Errorf("assColor(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{
		"tiếng Việt": "vi", "Vietnamese": "vi", "vi": "vi",
		"tiếng Anh": "en", "": "", "tự động": "", "không rõ": "",
	}
	for in, want := range cases {
		if got := NormalizeLang(in); got != want {
			t.Errorf("NormalizeLang(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestWordsFallbackSegment(t *testing.T) {
	tr := &Transcript{Segments: []Segment{
		{Start: 0, End: 1.5, Text: "không có mốc từng từ"},
		{Start: 2, End: 3, Text: "có mốc", Words: []Word{{Text: "có", Start: 2, End: 2.4}}},
	}}
	got := tr.Words()
	if len(got) != 2 || got[0].End != 1.5 || got[1].Text != "có" {
		t.Errorf("Words() = %+v", got)
	}
}

func TestSaveLoadJSON(t *testing.T) {
	path := t.TempDir() + "/tr.json"
	if err := SaveJSON(sampleTranscript(), path); err != nil {
		t.Fatalf("SaveJSON lỗi: %v", err)
	}
	got, err := LoadJSON(path)
	if err != nil {
		t.Fatalf("LoadJSON lỗi: %v", err)
	}
	if got.WordCount() != 6 || got.Language != "vi" {
		t.Errorf("transcript đọc lại sai: %d từ, lang %q", got.WordCount(), got.Language)
	}
}

// TestTranscribeSmoke — bóc băng THẬT. Chỉ chạy khi trỏ file audio qua biến môi
// trường, ví dụ: BIZSTUDIO_WHISPER_AUDIO=/tmp/vn.wav BIZSTUDIO_DATA=data go test ./internal/whisper/ -run Smoke -v
func TestTranscribeSmoke(t *testing.T) {
	src := os.Getenv("BIZSTUDIO_WHISPER_AUDIO")
	if src == "" {
		t.Skip("bỏ qua: đặt BIZSTUDIO_WHISPER_AUDIO=<file> để chạy thật")
	}
	dataDir := os.Getenv("BIZSTUDIO_DATA")
	if dataDir == "" {
		dataDir = "../../data"
	}
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("mở store lỗi: %v", err)
	}
	if !Available(st) {
		t.Skip("bỏ qua: chưa cài venv faster-whisper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	tr, err := Transcribe(ctx, st, src, "vi", func(p float64, d string) {
		t.Logf("%.0f%% — %s", p, d)
	})
	if err != nil {
		t.Fatalf("Transcribe lỗi: %v", err)
	}
	if tr.WordCount() == 0 {
		t.Fatal("không có mốc từng từ nào")
	}
	t.Logf("ngôn ngữ %s · %.2fs · %d đoạn · %d từ", tr.Language, tr.Duration, len(tr.Segments), tr.WordCount())
	for _, w := range tr.Words() {
		t.Logf("  %6.2f → %6.2f  %s", w.Start, w.End, w.Text)
	}
}
