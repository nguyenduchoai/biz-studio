package server

import (
	"net/http"
	"sync"
)

// Shared with the process-wide setup reservation: drain requests that may
// publish jobs or save uploads before checking whether restart is safe.
var updateMutationMu sync.RWMutex

func beginMutation(w http.ResponseWriter, r *http.Request, applying bool) (func(), bool) {
	var unlock func()
	if applying {
		updateMutationMu.Lock()
		unlock = updateMutationMu.Unlock
	} else {
		updateMutationMu.RLock()
		unlock = updateMutationMu.RUnlock
	}
	if r.Context().Err() != nil {
		unlock()
		httpErr(w, http.StatusRequestTimeout, "yêu cầu đã hủy trước khi thực hiện")
		return nil, false
	}
	if setupIsRunning("app-update") {
		unlock()
		httpErr(w, http.StatusConflict, "ứng dụng đang cập nhật; chờ khởi động lại rồi thử tiếp")
		return nil, false
	}
	return unlock, true
}

func updateAwareMobile(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			unlock, ok := beginMutation(w, r, false)
			if !ok {
				return
			}
			defer unlock()
		}
		next.ServeHTTP(w, r)
	})
}
