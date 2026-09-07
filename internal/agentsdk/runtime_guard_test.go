package agentsdk

import (
	"os"
	"os/exec"
	"testing"
)

func TestRuntimeInstallExcludesConcurrentSessionsAndProcesses(t *testing.T) {
	dir := t.TempDir()
	use, ok := TryUseRuntime(dir)
	if !ok {
		t.Fatal("first runtime reader blocked")
	}
	defer use()
	use2, ok := TryUseRuntime(dir)
	if !ok {
		t.Fatal("independent reader blocked")
	}
	use2()
	if release, ok := TryInstallRuntime(dir); ok {
		release()
		t.Fatal("installer acquired active runtime")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRuntimeLockSubprocess$")
	cmd.Env = append(os.Environ(), "BIZSTUDIO_LOCK_TEST_DIR="+dir)
	if err := cmd.Run(); err == nil {
		t.Fatal("second process replaced active runtime")
	}
	use()
	install, ok := TryInstallRuntime(dir)
	if !ok {
		t.Fatal("idle runtime cannot be installed")
	}
	if release, ok := TryUseRuntime(dir); ok {
		release()
		t.Fatal("session read half-installed runtime")
	}
	install()
	install()
	cmd = exec.Command(os.Args[0], "-test.run=^TestRuntimeLockSubprocess$")
	cmd.Env = append(os.Environ(), "BIZSTUDIO_LOCK_TEST_DIR="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("OS lock not released: %v %s", err, out)
	}
}

func TestRuntimeLockSubprocess(t *testing.T) {
	dir := os.Getenv("BIZSTUDIO_LOCK_TEST_DIR")
	if dir == "" {
		return
	}
	release, ok := TryInstallRuntime(dir)
	if !ok {
		os.Exit(4)
	}
	release()
}
