package setup

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestPythonRuntimeRejectsIncompatibleInterpreters(t *testing.T) {
	for _, tc := range []struct {
		name        string
		version     []int
		bits        int
		machine     string
		arm64, want bool
	}{
		{"managed 311", []int{3, 11, 9}, 64, "AMD64", false, true},
		{"supported 313", []int{3, 13, 1}, 64, "AMD64", false, true},
		{"old system Python", []int{3, 9, 6}, 64, "arm64", true, false},
		{"32-bit", []int{3, 11, 9}, 32, "x86", false, false},
		{"future Python", []int{4, 0, 0}, 64, "AMD64", false, false},
		{"unverified 314", []int{3, 14, 0}, 64, "AMD64", false, false},
		{"Rosetta-only Python", []int{3, 11, 9}, 64, "x86_64", true, false},
		{"native Apple Silicon", []int{3, 11, 9}, 64, "arm64", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"path": filepath.Join(t.TempDir(), "python"), "version": tc.version, "bits": tc.bits, "machine": tc.machine})
			py, err := parsePythonRuntime(data, tc.arm64)
			if (err == nil) != tc.want {
				t.Fatalf("runtime=%+v err=%v, want compatible=%v", py, err, tc.want)
			}
		})
	}
}

func TestPythonCandidatesPreferManaged311(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		candidates := pythonCandidates(goos)
		if goos == "windows" {
			if candidates[0].bin != "py" || candidates[0].args[0] != "-3.11" {
				t.Fatal(candidates[0])
			}
		} else if candidates[0].bin != "python3.11" {
			t.Fatal(candidates[0])
		}
	}
}
