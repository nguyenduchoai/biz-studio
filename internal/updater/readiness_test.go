package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"bizstudio/internal/util"
)

func TestUpdateReadinessChecksVersionAndDataAfterPortFallback(t *testing.T) {
	dataDir := t.TempDir()
	dataID := util.DataDirID(dataDir)
	version, responseDataID := "2.15.0", dataID
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/instance" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"app":"bizstudio","version":%q,"dataID":%q}`, version, responseDataID)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	stage := Stage{Tag: "v2.15.0", LaunchArgs: []string{"-port", u.Port(), "-data", dataDir}}
	if !updateReady(stage, srv.Client()) {
		t.Fatal("matching live instance should be ready")
	}
	version = "2.14.1"
	if updateReady(stage, srv.Client()) {
		t.Fatal("old version must not commit the update")
	}
	version, responseDataID = "2.15.0", "another-data-directory"
	if updateReady(stage, srv.Client()) {
		t.Fatal("different data directory must not commit the update")
	}
	responseDataID = dataID
	body, err := json.Marshal(map[string]string{"url": srv.URL, "dataID": dataID})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "instance.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	stage.LaunchArgs[1] = "1" // Preferred port changed; startup recorded a free port.
	if !updateReady(stage, srv.Client()) {
		t.Fatal("must discover the new listener from the instance marker")
	}
}

func TestRelaunchExitingBeforeReadyFails(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestUpdaterChildExit$")
	cmd.Env = append(os.Environ(), "BIZSTUDIO_UPDATE_PARENT_TEST=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := awaitRelaunch(cmd, Stage{}, 5*time.Second); err == nil {
		t.Fatal("process exiting without a ready API must fail, even with exit code zero")
	}
}
