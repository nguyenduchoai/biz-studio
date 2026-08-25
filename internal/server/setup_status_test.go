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
