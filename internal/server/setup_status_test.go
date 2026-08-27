package server

import "testing"

func TestPythonVersionSupported(t *testing.T) {
	cases := map[string]bool{
		"Python 3.9.18": false,
		"Python 3.10.0": true,
		"Python 3.11.9": true,
		"Python 4.0.0":  true,
		"không rõ":      false,
	}
	for version, want := range cases {
		if got := pythonVersionSupported(version); got != want {
			t.Errorf("pythonVersionSupported(%q) = %v, muốn %v", version, got, want)
		}
	}
}

func TestWindowsPythonCandidatesPrefer311Launcher(t *testing.T) {
	candidates := pythonCandidates("windows")
	if len(candidates) < 2 {
		t.Fatalf("thiếu Python candidates: %#v", candidates)
	}
	if candidates[0].bin != "py" || len(candidates[0].args) < 1 || candidates[0].args[0] != "-3.11" {
		t.Fatalf("candidate đầu = %#v, muốn py -3.11", candidates[0])
	}
	if candidates[1].bin != "py" || len(candidates[1].args) < 1 || candidates[1].args[0] != "-3" {
		t.Fatalf("candidate dự phòng = %#v, muốn py -3", candidates[1])
	}
}
