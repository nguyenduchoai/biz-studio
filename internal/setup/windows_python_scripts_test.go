package setup

import (
	"strings"
	"testing"
)

func TestWindowsPythonScriptsValidateInterpreterAndPrefer311(t *testing.T) {
	for _, name := range []string{"setup-whisper.ps1", "setup-vieneu.ps1"} {
		body, err := scriptFile(name)
		if err != nil {
			t.Fatal(err)
		}
		script := string(body)
		for _, want := range []string{
			`@("py", "-3.11")`,
			`sys.version_info >= (3, 10)`,
			`sys.maxsize > 2**32`,
			`$VenvPy -m pip`,
			`Python 3.10+ 64-bit`,
			`$env:PYTHONUTF8 = "1"`,
			`$env:PYTHONIOENCODING = "utf-8"`,
		} {
			if !strings.Contains(script, want) {
				t.Errorf("%s thiếu %q", name, want)
			}
		}
	}
}

func TestWhisperModelPrefetchIsOptionalAndDoesNotLoadRuntime(t *testing.T) {
	body, err := scriptFile("setup-whisper.ps1")
	if err != nil {
		t.Fatal(err)
	}
	script := string(body)
	for _, want := range []string{
		"from faster_whisper.utils import download_model",
		"HF_HUB_DISABLE_XET",
		"faster-whisper đã cài xong; model sẽ tải khi bóc băng lần đầu",
		"exit 0",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("setup-whisper.ps1 thiếu %q", want)
		}
	}
	if strings.Contains(script, "WhisperModel(") {
		t.Error("setup-whisper.ps1 đang nạp runtime model trong bộ cài thay vì chỉ tải trước")
	}
}

func TestWhisperManualFallbackDoesNotMisdiagnosePython(t *testing.T) {
	tool, ok := Find("whisper")
	if !ok {
		t.Fatal("không tìm thấy tool whisper")
	}
	if strings.Contains(tool.Manual, "python.org") {
		t.Fatalf("Whisper lỗi lại hướng người dùng cài Python: %s", tool.Manual)
	}
}
