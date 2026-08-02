package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Hub — SSE broadcaster đơn giản.
type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[chan []byte]struct{}{}}
}

// Broadcast gửi event tới mọi client đang kết nối.
func (h *Hub) Broadcast(event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, b))
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default: // client chậm — bỏ qua, không block
		}
	}
}

// ServeHTTP xử lý GET /api/events/stream.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		httpErr(w, 500, "streaming không được hỗ trợ")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
	}()

	_, _ = w.Write([]byte(": connected\n\n"))
	fl.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if _, err := w.Write(msg); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
