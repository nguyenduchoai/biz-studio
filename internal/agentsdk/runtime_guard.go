package agentsdk

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var runtimeGuards sync.Map

func runtimeGuard(dataDir string) *sync.RWMutex {
	key, _ := filepath.Abs(dataDir)
	if real, err := filepath.EvalSymlinks(key); err == nil {
		key = real
	}
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	guard, _ := runtimeGuards.LoadOrStore(key, &sync.RWMutex{})
	return guard.(*sync.RWMutex)
}

// TryUseRuntime holds the installed files stable until the whole SDK process
// tree exits. Independent projects may use the same runtime concurrently.
func TryUseRuntime(dataDir string) (func(), bool) {
	return tryRuntimeLock(dataDir, false)
}

// TryInstallRuntime prevents pip replacing files while an SDK invocation uses
// them, including the race between checking HTTP status and starting a process.
func TryInstallRuntime(dataDir string) (func(), bool) {
	return tryRuntimeLock(dataDir, true)
}

func tryRuntimeLock(dataDir string, exclusive bool) (func(), bool) {
	guard := runtimeGuard(dataDir)
	unlock := guard.RUnlock
	if exclusive {
		if !guard.TryLock() {
			return nil, false
		}
		unlock = guard.Unlock
	} else if !guard.TryRLock() {
		return nil, false
	}
	dir := filepath.Join(dataDir, "agent-sdk")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		unlock()
		return nil, false
	}
	file, err := os.OpenFile(filepath.Join(dir, "runtime.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		unlock()
		return nil, false
	}
	if err := lockRuntimeFile(file, exclusive); err != nil {
		file.Close()
		unlock()
		return nil, false
	}
	var once sync.Once
	// The OS lease also covers separate `bizstudio setup` processes, and is
	// released by the OS after a crash. Never unlink a lock file with waiters.
	return func() { once.Do(func() { unlockRuntimeFile(file); file.Close(); unlock() }) }, true
}
