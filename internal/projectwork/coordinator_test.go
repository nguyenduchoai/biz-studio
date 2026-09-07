package projectwork

import "testing"

func TestActiveCountTracksOwnershipUntilRelease(t *testing.T) {
	var c Coordinator
	if c.ActiveCount() != 0 {
		t.Fatal("unused coordinator should be idle")
	}
	release, ok := c.TryAcquire("project-a")
	if !ok || c.ActiveCount() != 1 {
		t.Fatal("held project lease must count as active work")
	}
	release()
	if c.ActiveCount() != 0 {
		t.Fatal("released lease should no longer prevent shutdown")
	}
}

func TestCoordinatorBroadcastsReleaseAndKeepsReplacementLease(t *testing.T) {
	var c Coordinator // zero value is usable, too
	release, ok := c.TryAcquire("project-a")
	if !ok {
		t.Fatal("first lease rejected")
	}
	if _, ok := c.TryAcquire("project-a"); ok {
		t.Fatal("same project acquired twice")
	}
	firstWaiter, secondWaiter := c.Changed(), c.Changed()
	release()
	for _, waiter := range []<-chan struct{}{firstWaiter, secondWaiter} {
		select {
		case <-waiter:
		default:
			t.Fatal("release did not broadcast to every waiter")
		}
	}
	replacement, ok := c.TryAcquire("project-a")
	if !ok {
		t.Fatal("released project did not become available")
	}
	changed := c.Changed()
	release() // retrying an old release must not unlock the replacement holder
	select {
	case <-changed:
		t.Fatal("duplicate release broadcast a false change")
	default:
	}
	if _, ok := c.TryAcquire("project-a"); ok {
		t.Fatal("old release unlocked replacement lease")
	}
	replacement()
}

func TestCoordinatorAllowsOtherAndUnscopedProjects(t *testing.T) {
	c := New()
	for _, id := range []string{"project-a", "project-b", "", ""} {
		release, ok := c.TryAcquire(id)
		if !ok || release == nil {
			t.Fatalf("independent key %q rejected", id)
		}
		defer release()
	}
}
