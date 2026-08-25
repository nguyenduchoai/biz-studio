package server

import (
	"testing"
	"time"
)

func TestFullSetupGrantIsExactOneTimeAndExpires(t *testing.T) {
	s := newTestServer(t)
	now := time.Unix(1_800_000_000, 0)
	id := s.createFullSetupGrant([]string{"git", "claude"}, now)
	got, ok := s.consumeFullSetupGrant(id, now.Add(time.Minute))
	if !ok || len(got) != 2 || got[0] != "git" || got[1] != "claude" {
		t.Fatalf("grant không giữ đúng plan: %#v ok=%v", got, ok)
	}
	if _, ok := s.consumeFullSetupGrant(id, now.Add(time.Minute)); ok {
		t.Fatal("grant đã dùng vẫn dùng lại được")
	}
	expired := s.createFullSetupGrant([]string{"git"}, now)
	if _, ok := s.consumeFullSetupGrant(expired, now.Add(fullPlanTTL+time.Second)); ok {
		t.Fatal("grant hết hạn vẫn dùng được")
	}
}
