package setup

import (
	"fmt"
	"strings"
	"testing"
)

func TestAppleSiliconBrewNeverFallsBackToIntelPrefix(t *testing.T) {
	step, err := nativeBrewStep(true, func(path string) (string, error) {
		if path == "brew" {
			return "/usr/local/bin/brew", nil
		}
		return "", fmt.Errorf("native Homebrew absent")
	})
	if err == nil || !strings.Contains(err.Error(), "Homebrew bản arm64") {
		t.Fatalf("Intel-only Mac must explain native prerequisite: step=%+v err=%v", step, err)
	}
}

func TestAppleSiliconBrewForcesNativeProcessUnderRosetta(t *testing.T) {
	step, err := nativeBrewStep(true, func(path string) (string, error) { return path, nil })
	if err != nil || step.Bin != "/usr/bin/arch" || strings.Join(step.Args, " ") != "-arm64 /opt/homebrew/bin/brew" {
		t.Fatalf("native brew invocation: %+v, %v", step, err)
	}
}

func TestIntelBrewUsesExistingPATH(t *testing.T) {
	step, err := nativeBrewStep(false, func(path string) (string, error) { return "/usr/local/bin/" + path, nil })
	if err != nil || step.Bin != "/usr/local/bin/brew" || len(step.Args) != 0 {
		t.Fatalf("Intel brew invocation: %+v, %v", step, err)
	}
}
