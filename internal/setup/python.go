package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PythonRuntime là interpreter đã chạy thật, cùng kiến trúc với runner media.
type PythonRuntime struct {
	Path    string
	Version string
}

type pythonCandidate struct {
	bin  string
	args []string
}

const pythonProbe = `import json, sys, platform, venv; print(json.dumps({"path":sys.executable,"version":list(sys.version_info[:3]),"bits":64 if sys.maxsize > 2**32 else 32,"machine":platform.machine()}))`

// FindPython dùng chung cho trạng thái và bộ cài để tránh báo đã có Python
// nhưng tạo venv bằng một bản khác. Python 3.11 là bản Full quản lý; 3.10–3.13
// là các bản được VieNeu 3.2.3 công bố hỗ trợ. Không mở Windows Store aliases.
func FindPython(ctx context.Context) (PythonRuntime, error) {
	arm64 := nativePythonARM64(ctx)
	for _, c := range pythonCandidates(runtime.GOOS) {
		if ctx.Err() != nil {
			return PythonRuntime{}, ctx.Err()
		}
		path, err := exec.LookPath(c.bin)
		if err != nil || strings.Contains(strings.ToLower(path), `\microsoft\windowsapps\python`) {
			continue
		}
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		args := append(append([]string{}, c.args...), "-I", "-c", pythonProbe)
		if arm64 {
			args = append([]string{"-arm64", path}, args...)
			path = "/usr/bin/arch"
		}
		out, err := exec.CommandContext(probeCtx, path, args...).Output()
		cancel()
		if err == nil {
			if py, err := parsePythonRuntime(out, arm64); err == nil {
				return py, nil
			}
		}
	}
	return PythonRuntime{}, fmt.Errorf("cần Python 3.10–3.13 bản 64-bit (bộ cài Full dùng Python 3.11); hãy cài Python 3.11 rồi kiểm tra lại")
}

func parsePythonRuntime(out []byte, arm64 bool) (PythonRuntime, error) {
	var p struct {
		Path    string `json:"path"`
		Version []int  `json:"version"`
		Bits    int    `json:"bits"`
		Machine string `json:"machine"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		return PythonRuntime{}, err
	}
	if !filepath.IsAbs(p.Path) || len(p.Version) != 3 || p.Version[0] != 3 || p.Version[1] < 10 || p.Version[1] > 13 || p.Bits != 64 {
		return PythonRuntime{}, fmt.Errorf("Python không tương thích")
	}
	if arm64 && !strings.EqualFold(p.Machine, "arm64") && !strings.EqualFold(p.Machine, "aarch64") {
		return PythonRuntime{}, fmt.Errorf("cần Python arm64 trên Apple Silicon")
	}
	return PythonRuntime{Path: p.Path, Version: fmt.Sprintf("Python %d.%d.%d · 64-bit", p.Version[0], p.Version[1], p.Version[2])}, nil
}

func pythonCandidates(goos string) []pythonCandidate {
	var candidates []pythonCandidate
	if goos == "windows" {
		candidates = append(candidates, pythonCandidate{"py", []string{"-3.11"}})
		for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles")} {
			if base == "" {
				continue
			}
			for _, version := range []string{"311", "312", "310", "313"} {
				candidates = append(candidates, pythonCandidate{filepath.Join(base, "Programs", "Python", "Python"+version, "python.exe"), nil},
					pythonCandidate{filepath.Join(base, "Python"+version, "python.exe"), nil})
			}
		}
		for _, version := range []string{"-3.12", "-3.10", "-3.13", "-3"} {
			candidates = append(candidates, pythonCandidate{"py", []string{version}})
		}
	} else {
		for _, version := range []string{"3.11", "3.12", "3.10", "3.13"} {
			candidates = append(candidates, pythonCandidate{"python" + version, nil})
			if goos == "darwin" {
				for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
					candidates = append(candidates, pythonCandidate{prefix + "/opt/python@" + version + "/bin/python" + version, nil})
				}
				candidates = append(candidates, pythonCandidate{"/Library/Frameworks/Python.framework/Versions/" + version + "/bin/python" + version, nil})
			}
		}
	}
	return append(candidates, pythonCandidate{"python3", nil}, pythonCandidate{"python", nil})
}

func nativePythonARM64(ctx context.Context) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	out, err := exec.CommandContext(probeCtx, "/usr/sbin/sysctl", "-n", "hw.optional.arm64").Output()
	return err == nil && strings.TrimSpace(string(out)) == "1"
}
