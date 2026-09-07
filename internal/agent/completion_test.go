package agent

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"bizstudio/internal/store"
)

func testRunner(t *testing.T) (*Runner, store.Project, store.Session) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := store.Project{Name: "Video", Status: "running"}
	st.SaveProject(&p)
	s := store.Session{ProjectID: p.ID, Status: "running"}
	st.SaveSession(&s)
	return New(st, func(string, any) {}, st.DataDir), p, s
}

func TestResultCannotCompleteBeforeProcessExit(t *testing.T) {
	r, p, s := testRunner(t)
	r.handleLine(s.ID, p.ID, []byte(`{"type":"result","subtype":"success","num_turns":2}`))
	s, _ = r.st.Session(s.ID)
	p, _ = r.st.Project(p.ID)
	if s.Status != "running" || p.Status != "running" || !s.EndedAt.IsZero() {
		t.Fatalf("early result finalized unfinished process: session=%+v project=%+v", s, p)
	}
}

func TestSuccessResultFollowedByProcessErrorFails(t *testing.T) {
	r, p, s := testRunner(t)
	r.handleLine(s.ID, p.ID, []byte(`{"type":"result","subtype":"success"}`))
	r.finishRun(context.Background(), s.ID, p.ID, &streamEvent{Subtype: "success"}, errors.New("exit status 7"), &bytes.Buffer{}, nil)
	s, _ = r.st.Session(s.ID)
	p, _ = r.st.Project(p.ID)
	if s.Status != "error" || p.Status != "error" {
		t.Fatalf("nonzero exit accepted: session=%s project=%s", s.Status, p.Status)
	}
}

func TestMissingOrFailedResultCannotComplete(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result *streamEvent
	}{
		{"missing", nil},
		{"is-error", &streamEvent{Subtype: "success", IsError: true}},
		{"budget", &streamEvent{Subtype: "error_max_budget_usd"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, p, s := testRunner(t)
			r.validateVideo = func(context.Context, string) error { t.Error("invalid result reached output gate"); return nil }
			r.finishRun(context.Background(), s.ID, p.ID, tc.result, nil, &bytes.Buffer{}, nil)
			s, _ = r.st.Session(s.ID)
			p, _ = r.st.Project(p.ID)
			if s.Status != "error" || p.Status != "error" {
				t.Fatalf("result accepted: %s/%s", s.Status, p.Status)
			}
		})
	}
}

func TestSuccessfulProcessWithoutOutputIsError(t *testing.T) {
	r, p, s := testRunner(t)
	r.finishRun(context.Background(), s.ID, p.ID, &streamEvent{Subtype: "success"}, nil, &bytes.Buffer{}, nil)
	s, _ = r.st.Session(s.ID)
	p, _ = r.st.Project(p.ID)
	if s.Status != "error" || p.Status != "error" {
		t.Fatalf("missing output accepted: %s/%s", s.Status, p.Status)
	}
}
