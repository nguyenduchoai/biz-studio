// Package projectwork coordinates jobs and AI sessions which write the same
// project's files. It is independent of jobs/store to let both share a lease.
package projectwork

import "sync"

type Coordinator struct {
	mu      sync.Mutex
	active  map[string]bool
	changed chan struct{}
}

func New() *Coordinator {
	return &Coordinator{active: make(map[string]bool), changed: make(chan struct{})}
}

// TryAcquire never waits. A successful release is idempotent. Unscoped work
// (empty project ID) is independent and does not contend for a shared key.
func (c *Coordinator) TryAcquire(projectID string) (release func(), ok bool) {
	if projectID == "" {
		return func() {}, true
	}
	c.mu.Lock()
	c.initLocked()
	if c.active[projectID] {
		c.mu.Unlock()
		return nil, false
	}
	c.active[projectID] = true
	c.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			delete(c.active, projectID)
			close(c.changed)
			c.changed = make(chan struct{})
		})
	}, true
}

// Changed broadcasts the next release to every waiter. Obtain this channel
// BEFORE attempting acquisitions so a release during a scan is not missed.
func (c *Coordinator) Changed() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	return c.changed
}

// ActiveCount reflects ownership, not UI status. A stopped AI session may still
// own its lease while its subprocess tree exits and output readers drain.
func (c *Coordinator) ActiveCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.active)
}

func (c *Coordinator) initLocked() {
	if c.active == nil {
		c.active = make(map[string]bool)
	}
	if c.changed == nil {
		c.changed = make(chan struct{})
	}
}
