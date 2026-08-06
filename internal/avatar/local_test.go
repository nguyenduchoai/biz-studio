package avatar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// LongCat KHÔNG nhận ảnh/giọng qua tham số dòng lệnh mà đọc từ một file JSON.
// Sai một tên trường là model chạy vài phút rồi mới báo lỗi, nên định dạng này
// được khoá cứng theo đúng file mẫu của repo (assets/avatar/single_example_1.json).
func TestWriteInputJSONDungDinhDang(t *testing.T) {
	dir := t.TempDir()
	p, err := writeInputJSON(dir, Opts{
		ImagePath: "/data/face.png",
		AudioPath: "/data/voice.wav",
		Prompt:    "A person speaking",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("file sinh ra không phải JSON hợp lệ: %v", err)
	}
	if got["prompt"] != "A person speaking" {
		t.Errorf("prompt = %v", got["prompt"])
	}
	if got["cond_image"] != "/data/face.png" {
		t.Errorf("cond_image = %v", got["cond_image"])
	}
	ca, ok := got["cond_audio"].(map[string]any)
	if !ok {
		t.Fatalf("cond_audio phải là object, nhận %T", got["cond_audio"])
	}
	if ca["person1"] != "/data/voice.wav" {
		t.Errorf("cond_audio.person1 = %v", ca["person1"])
	}
	if filepath.Base(p) != "input.json" {
		t.Errorf("tên file = %s", filepath.Base(p))
	}
}

// Prompt rỗng phải được thay bằng mô tả mặc định — để rỗng thì model tự bịa
// bối cảnh, hay ra kết quả lệch hẳn ý người dùng.
func TestPromptRongDungMacDinh(t *testing.T) {
	dir := t.TempDir()
	p, _ := writeInputJSON(dir, Opts{ImagePath: "a.png", AudioPath: "b.wav", Prompt: "   "})
	raw, _ := os.ReadFile(p)
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	if got["prompt"] != DefaultPrompt {
		t.Errorf("prompt rỗng phải thành mặc định, nhận: %v", got["prompt"])
	}
}

// Nhiều GPU thì phải thêm --context_parallel_size, và cờ này phải đứng TRƯỚC
// tên script (torchrun ăn tham số của nó trước, sau đó mới tới script).
func TestTorchrunArgs(t *testing.T) {
	one := torchrunArgs(cfg{gpus: 1, checkpoint: "/w"}, "/in.json", "/out")
	if idxOf(one, "--context_parallel_size=1") >= 0 {
		t.Error("1 GPU thì không cần --context_parallel_size")
	}
	if idxOf(one, "--use_int8") >= 0 {
		t.Error("không bật int8 thì không được thêm cờ")
	}

	two := torchrunArgs(cfg{gpus: 2, checkpoint: "/w", int8: true}, "/in.json", "/out")
	cp := idxOf(two, "--context_parallel_size=2")
	sc := idxOf(two, demoScript)
	if cp < 0 {
		t.Fatal("2 GPU phải có --context_parallel_size=2")
	}
	if cp > sc {
		t.Errorf("--context_parallel_size phải đứng trước %s (vị trí %d > %d)", demoScript, cp, sc)
	}
	if idxOf(two, "--use_int8") < 0 {
		t.Error("bật int8 phải có cờ --use_int8")
	}
	if idxOf(two, "--nproc_per_node=2") != 0 {
		t.Error("--nproc_per_node phải là tham số đầu tiên của torchrun")
	}
}

// Thiếu ảnh hay giọng phải chặn NGAY, không để chạy vài phút rồi mới hỏng.
func TestValidate(t *testing.T) {
	if validate(Opts{AudioPath: "x.wav"}) == nil {
		t.Error("thiếu ảnh phải báo lỗi")
	}
	if validate(Opts{ImagePath: "x.png"}) == nil {
		t.Error("thiếu giọng phải báo lỗi")
	}
	if validate(Opts{ImagePath: "/khong/co/that.png", AudioPath: "/khong/co/that.wav"}) == nil {
		t.Error("file không tồn tại phải báo lỗi")
	}
}

func idxOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}
