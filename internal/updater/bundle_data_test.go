package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartRejectsDataInsideMacBundleBeforeLaunchingHelper(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Biz Studio.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{app, filepath.Join(app, "Contents", "data"), filepath.Join(app, "new-data")} {
		stage := Stage{Kind: "tar-app", Target: app, Archive: "unused.tar.gz", LaunchArgs: []string{"-data", data}}
		if err := Start(stage); err == nil || !strings.Contains(err.Error(), "dữ liệu đang nằm") {
			t.Fatalf("contained data %s was not rejected before helper creation: %v", data, err)
		}
	}
	outside := filepath.Join(filepath.Dir(app), "Biz Studio.app-data")
	if err := validateDataOutsideBundle(Stage{Kind: "tar-app", Target: app, LaunchArgs: []string{"-data", outside}}); err != nil {
		t.Fatalf("sibling directory must stay valid: %v", err)
	}
}

func TestMacBundleDataCheckResolvesSymlinkAliases(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Biz Studio.app")
	inside := filepath.Join(app, "data")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "data-alias")
	if err := os.Symlink(inside, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	stage := Stage{Kind: "tar-app", Target: app, LaunchArgs: []string{"-data", alias}}
	if err := validateDataOutsideBundle(stage); err == nil {
		t.Fatal("symlink pointing into application bundle must be rejected")
	}
}
