package store

import "sort"

// ---------- Projects ----------

func (s *Store) Projects() []Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Project(nil), s.d.Projects...)
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func (s *Store) Project(id string) (Project, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.d.Projects {
		if p.ID == id {
			return p, true
		}
	}
	return Project{}, false
}

func (s *Store) SaveProject(p *Project) {
	if p.ID == "" {
		p.ID = s.NewID("prj")
		p.CreatedAt = now()
	}
	p.UpdatedAt = now()
	s.write(func(d *db) {
		for i := range d.Projects {
			if d.Projects[i].ID == p.ID {
				d.Projects[i] = *p
				return
			}
		}
		d.Projects = append(d.Projects, *p)
	})
}

func (s *Store) DeleteProject(id string) {
	s.write(func(d *db) {
		d.Projects = filterSlice(d.Projects, func(p Project) bool { return p.ID != id })
		d.Assets = filterSlice(d.Assets, func(a Asset) bool { return a.ProjectID != id })
		keep := map[string]bool{}
		for _, ss := range d.Sessions {
			if ss.ProjectID == id {
				keep[ss.ID] = true
			}
		}
		d.Sessions = filterSlice(d.Sessions, func(x Session) bool { return x.ProjectID != id })
		d.Events = filterSlice(d.Events, func(e SessionEvent) bool { return !keep[e.SessionID] })
	})
}

// ---------- Assets ----------

func (s *Store) AssetsByProject(pid string) []Asset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Asset
	for _, a := range s.d.Assets {
		if a.ProjectID == pid {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *Store) Asset(id string) (Asset, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.d.Assets {
		if a.ID == id {
			return a, true
		}
	}
	return Asset{}, false
}

func (s *Store) SaveAsset(a *Asset) {
	if a.ID == "" {
		a.ID = s.NewID("ast")
		a.CreatedAt = now()
	}
	s.write(func(d *db) {
		for i := range d.Assets {
			if d.Assets[i].ID == a.ID {
				d.Assets[i] = *a
				return
			}
		}
		d.Assets = append(d.Assets, *a)
	})
}

func (s *Store) DeleteAsset(id string) {
	s.write(func(d *db) {
		d.Assets = filterSlice(d.Assets, func(a Asset) bool { return a.ID != id })
	})
}

// ---------- Sessions & Events ----------

func (s *Store) SessionsByProject(pid string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Session
	for _, x := range s.d.Sessions {
		if x.ProjectID == pid {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}

func (s *Store) Session(id string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, x := range s.d.Sessions {
		if x.ID == id {
			return x, true
		}
	}
	return Session{}, false
}

func (s *Store) SaveSession(x *Session) {
	if x.ID == "" {
		x.ID = s.NewID("ses")
		x.StartedAt = now()
	}
	s.write(func(d *db) {
		for i := range d.Sessions {
			if d.Sessions[i].ID == x.ID {
				d.Sessions[i] = *x
				return
			}
		}
		d.Sessions = append(d.Sessions, *x)
	})
}

func (s *Store) AddEvent(e *SessionEvent) {
	if e.ID == "" {
		e.ID = s.NewID("evt")
		e.CreatedAt = now()
	}
	s.write(func(d *db) { d.Events = append(d.Events, *e) })
}

func (s *Store) EventsBySession(sid string) []SessionEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []SessionEvent
	for _, e := range s.d.Events {
		if e.SessionID == sid {
			out = append(out, e)
		}
	}
	return out
}

// ---------- Jobs ----------

func (s *Store) Jobs() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Job(nil), s.d.Jobs...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) Job(id string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, j := range s.d.Jobs {
		if j.ID == id {
			return j, true
		}
	}
	return Job{}, false
}

func (s *Store) SaveJob(j *Job) {
	if j.ID == "" {
		j.ID = s.NewID("job")
		j.CreatedAt = now()
	}
	j.UpdatedAt = now()
	s.write(func(d *db) {
		for i := range d.Jobs {
			if d.Jobs[i].ID == j.ID {
				d.Jobs[i] = *j
				return
			}
		}
		d.Jobs = append(d.Jobs, *j)
	})
}

// ---------- Characters ----------

func (s *Store) Characters() []Character {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Character(nil), s.d.Chars...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *Store) Character(id string) (Character, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.d.Chars {
		if c.ID == id {
			return c, true
		}
	}
	return Character{}, false
}

func (s *Store) SaveCharacter(c *Character) {
	if c.ID == "" {
		c.ID = s.NewID("chr")
		c.CreatedAt = now()
	}
	c.UpdatedAt = now()
	s.write(func(d *db) {
		for i := range d.Chars {
			if d.Chars[i].ID == c.ID {
				d.Chars[i] = *c
				return
			}
		}
		d.Chars = append(d.Chars, *c)
	})
}

func (s *Store) DeleteCharacter(id string) {
	s.write(func(d *db) {
		d.Chars = filterSlice(d.Chars, func(c Character) bool { return c.ID != id })
	})
}

// ---------- Style kits ----------

func (s *Store) StyleKits() []StyleKit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]StyleKit(nil), s.d.Styles...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDefault != out[j].IsDefault {
			return out[i].IsDefault
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func (s *Store) StyleKit(id string) (StyleKit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.d.Styles {
		if k.ID == id {
			return k, true
		}
	}
	return StyleKit{}, false
}

// ActiveStyleKit trả bộ style đang đặt mặc định (không có → false).
func (s *Store) ActiveStyleKit() (StyleKit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.d.Styles {
		if k.IsDefault {
			return k, true
		}
	}
	return StyleKit{}, false
}

// SaveStyleKit lưu bộ style; đặt IsDefault sẽ tự bỏ mặc định ở các bộ khác.
func (s *Store) SaveStyleKit(k *StyleKit) {
	if k.ID == "" {
		k.ID = s.NewID("sty")
		k.CreatedAt = now()
	}
	k.UpdatedAt = now()
	s.write(func(d *db) {
		if k.IsDefault {
			for i := range d.Styles {
				if d.Styles[i].ID != k.ID {
					d.Styles[i].IsDefault = false
				}
			}
		}
		for i := range d.Styles {
			if d.Styles[i].ID == k.ID {
				d.Styles[i] = *k
				return
			}
		}
		d.Styles = append(d.Styles, *k)
	})
}

func (s *Store) DeleteStyleKit(id string) {
	s.write(func(d *db) {
		d.Styles = filterSlice(d.Styles, func(k StyleKit) bool { return k.ID != id })
	})
}

// ---------- Text → Video sessions ----------

func (s *Store) T2VSessions() []T2VSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]T2VSession(nil), s.d.T2V...)
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func (s *Store) T2VSession(id string) (T2VSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, x := range s.d.T2V {
		if x.ID == id {
			return x, true
		}
	}
	return T2VSession{}, false
}

func (s *Store) SaveT2VSession(x *T2VSession) {
	if x.ID == "" {
		x.ID = s.NewID("t2v")
		x.CreatedAt = now()
	}
	x.UpdatedAt = now()
	s.write(func(d *db) {
		for i := range d.T2V {
			if d.T2V[i].ID == x.ID {
				d.T2V[i] = *x
				return
			}
		}
		d.T2V = append(d.T2V, *x)
	})
}

func (s *Store) DeleteT2VSession(id string) {
	s.write(func(d *db) {
		d.T2V = filterSlice(d.T2V, func(x T2VSession) bool { return x.ID != id })
	})
}

// ---------- Clone voices ----------

func (s *Store) CloneVoices() []CloneVoice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]CloneVoice(nil), s.d.Clones...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) CloneVoice(id string) (CloneVoice, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.d.Clones {
		if c.ID == id {
			return c, true
		}
	}
	return CloneVoice{}, false
}

func (s *Store) SaveCloneVoice(c *CloneVoice) {
	if c.ID == "" {
		c.ID = s.NewID("clv")
		c.CreatedAt = now()
	}
	s.write(func(d *db) {
		for i := range d.Clones {
			if d.Clones[i].ID == c.ID {
				d.Clones[i] = *c
				return
			}
		}
		d.Clones = append(d.Clones, *c)
	})
}

func (s *Store) DeleteCloneVoice(id string) {
	s.write(func(d *db) {
		d.Clones = filterSlice(d.Clones, func(c CloneVoice) bool { return c.ID != id })
	})
}

// ---------- Prompts ----------

func (s *Store) Prompts() []PromptTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]PromptTemplate(nil), s.d.Prompts...)
}

func (s *Store) SavePrompt(p *PromptTemplate) {
	if p.ID == "" {
		p.ID = s.NewID("pmt")
	}
	s.write(func(d *db) {
		for i := range d.Prompts {
			if d.Prompts[i].ID == p.ID {
				d.Prompts[i] = *p
				return
			}
		}
		d.Prompts = append(d.Prompts, *p)
	})
}

func (s *Store) DeletePrompt(id string) {
	s.write(func(d *db) {
		d.Prompts = filterSlice(d.Prompts, func(p PromptTemplate) bool { return p.ID != id })
	})
}

// ---------- Logs & Settings ----------

func (s *Store) AddLog(level, module, msg string) LogEntry {
	e := LogEntry{ID: s.NewID("log"), Level: level, Module: module, Message: msg, CreatedAt: now()}
	s.write(func(d *db) {
		d.LogList = append(d.LogList, e)
		if len(d.LogList) > 2000 {
			d.LogList = d.LogList[len(d.LogList)-2000:]
		}
	})
	return e
}

func (s *Store) Logs(limit int) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.d.LogList)
	if limit <= 0 || limit > n {
		limit = n
	}
	out := append([]LogEntry(nil), s.d.LogList[n-limit:]...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (s *Store) Settings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.d.Config
}

func (s *Store) SaveSettings(c Settings) {
	s.write(func(d *db) { d.Config = c })
	s.mu.Lock()
	s.applyDefaults()
	_ = s.saveLocked()
	s.mu.Unlock()
}

func filterSlice[T any](in []T, keep func(T) bool) []T {
	out := in[:0]
	for _, v := range in {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}
