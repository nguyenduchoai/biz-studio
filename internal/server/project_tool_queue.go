package server

import (
	"os"
	"path/filepath"
	"strings"
)

// projectIDForToolPath maps a canonical contained path to an existing project.
// Tools which write siblings pass the containing directory, so a symlink on
// the source file itself cannot hide the project that owns the new files.
// Global/library paths stay unscoped. This is a single-primary-project lease,
// not a multi-project read/write lock for optional secondary media inputs.
func (s *Server) projectIDForToolPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	base, err := filepath.EvalSymlinks(filepath.Join(s.DataDir, "projects"))
	if err != nil {
		return ""
	}
	real, err := filepath.EvalSymlinks(s.toolAbsPath(path))
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(base, real)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return ""
	}
	id := strings.Split(filepath.ToSlash(rel), "/")[0]
	if p, ok := s.st.Project(id); ok {
		return p.ID
	}
	// Windows and common macOS filesystems permit differently-cased paths;
	// SameFile avoids guessing case-insensitivity on case-sensitive volumes.
	actual, err := os.Stat(filepath.Join(base, id))
	if err != nil {
		return ""
	}
	for _, p := range s.st.Projects() {
		if !strings.EqualFold(p.ID, id) {
			continue
		}
		known, err := os.Stat(filepath.Join(base, p.ID))
		if err == nil && os.SameFile(actual, known) {
			return p.ID
		}
	}
	return ""
}
