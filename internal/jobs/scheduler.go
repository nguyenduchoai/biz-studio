package jobs

// schedule scans the bounded FIFO for eligible project keys. Busy projects
// stay queued instead of occupying workers and blocking unrelated projects.
func (m *Manager) schedule() {
	for {
		changed := m.st.ProjectWork.Changed()
		m.mu.Lock()
		for m.active < m.limit {
			index, release := m.nextEligible()
			if index < 0 {
				break
			}
			item := m.pending[index]
			copy(m.pending[index:], m.pending[index+1:])
			m.pending[len(m.pending)-1] = queuedJob{}
			m.pending = m.pending[:len(m.pending)-1]
			m.active++
			go m.execute(item, release)
		}
		m.mu.Unlock()
		select {
		case <-m.wake:
		case <-changed:
		}
	}
}

// Called only while m.mu is held. Choosing the first acquirable item preserves
// FIFO within each project even when later, unrelated jobs bypass a busy key.
func (m *Manager) nextEligible() (int, func()) {
	for i, item := range m.pending {
		if release, ok := m.st.ProjectWork.TryAcquire(item.job.ProjectID); ok {
			return i, release
		}
	}
	return -1, nil
}

func (m *Manager) execute(item queuedJob, release func()) {
	defer func() {
		release()
		m.mu.Lock()
		m.active--
		m.mu.Unlock()
		m.notify()
	}()
	m.run(item)
}

func (m *Manager) notify() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}
