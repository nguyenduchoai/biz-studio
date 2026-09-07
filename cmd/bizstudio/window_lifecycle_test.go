package main

import (
	"errors"
	"testing"
	"time"

	"bizstudio/internal/store"
)

func TestWindowProcessHandoffDoesNotCloseServer(t *testing.T) {
	if windowProcessClosed(nil, 100*time.Millisecond) {
		t.Fatal("Chromium handing off to its existing profile must not terminate Biz Studio")
	}
	if windowProcessClosed(errors.New("browser crashed"), time.Minute) {
		t.Fatal("browser failure must keep the server available for the fallback browser")
	}
	if !windowProcessClosed(nil, time.Minute) {
		t.Fatal("a normally closed owned browser must still allow normal app shutdown")
	}
}

func TestReopeningWindowSupersedesOldShutdownWatcher(t *testing.T) {
	dir := t.TempDir()
	original := recordWindowLaunch(dir)
	if original.generation == "" || original.superseded() {
		t.Fatal("first window should own normal shutdown while it is the only launch")
	}
	reopened := recordWindowLaunch(dir)
	if !original.superseded() {
		t.Fatal("old watcher must not shut down a window reopened while a render finishes")
	}
	if reopened.superseded() {
		t.Fatal("new generation was not published")
	}
	if !(windowLaunch{dataDir: dir}).superseded() {
		t.Fatal("an unrecorded browser launch cannot safely authorize process exit")
	}
}

func TestRunningWorkKeepsAISessionAndProductionAlive(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := store.Project{Name: "Đang dựng"}
	st.SaveProject(&project)
	session := store.Session{ProjectID: project.ID, Status: "running"}
	st.SaveSession(&session)
	if runningWork(st) != 1 {
		t.Fatal("an AI session runs independently of Jobs and must keep the server alive")
	}
	session.Status = "done"
	st.SaveSession(&session)
	idea := store.Idea{Title: "Video", Status: "producing"}
	st.SaveIdea(&idea)
	if runningWork(st) != 1 {
		t.Fatal("an active idea production must keep the server alive")
	}
	idea.Status = "queued"
	st.SaveIdea(&idea)
	if runningWork(st) != 0 {
		t.Fatal("an unstarted idea queue and completed sessions must not prevent app shutdown")
	}
	job := store.Job{Status: "queued"}
	st.SaveJob(&job)
	if runningWork(st) != 1 {
		t.Fatal("a scheduled job must still keep the server alive")
	}
}

func TestRecoverInterruptedWorkPreservesHistoryAndCompletedResults(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := store.Project{Name: "Video", Status: "running", OutputFile: "outputs/existing.mp4"}
	st.SaveProject(&project)
	running := store.Session{ProjectID: project.ID, Status: "running", ClaudeSessionID: "resume-me"}
	st.SaveSession(&running)
	done := store.Session{ProjectID: project.ID, Status: "done"}
	st.SaveSession(&done)
	idea := store.Idea{Title: "Draft", Status: "producing", T2VSessionID: "saved-session"}
	st.SaveIdea(&idea)

	recoverInterruptedWork(st)
	got, _ := st.Session(running.ID)
	if got.Status != "stopped" || got.EndedAt.IsZero() || got.ClaudeSessionID != "resume-me" {
		t.Fatalf("interrupted session cannot be resumed safely: %#v", got)
	}
	completed, _ := st.Session(done.ID)
	if completed.Status != "done" {
		t.Fatal("recovery changed a completed session")
	}
	updatedProject, _ := st.Project(project.ID)
	if updatedProject.Status != "draft" || updatedProject.OutputFile != project.OutputFile {
		t.Fatalf("recovery lost project output or left it running: %#v", updatedProject)
	}
	updatedIdea, _ := st.Idea(idea.ID)
	if updatedIdea.Status != "error" || updatedIdea.Error == "" || updatedIdea.T2VSessionID != "saved-session" {
		t.Fatalf("recovery lost idea work or did not explain interruption: %#v", updatedIdea)
	}
	if runningWork(st) != 0 {
		t.Fatal("stale work still prevents shutdown after restart")
	}
	events := len(st.EventsBySession(running.ID))
	recoverInterruptedWork(st)
	if len(st.EventsBySession(running.ID)) != events {
		t.Fatal("repeated recovery added duplicate interruption events")
	}
}
