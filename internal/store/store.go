package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type db struct {
	Projects []Project        `json:"projects"`
	Assets   []Asset          `json:"assets"`
	Sessions []Session        `json:"sessions"`
	Events   []SessionEvent   `json:"events"`
	Jobs     []Job            `json:"jobs"`
	Ideas    []Idea           `json:"ideas"`
	Chars    []Character      `json:"characters"`
	Styles   []StyleKit       `json:"styleKits"`
	T2V      []T2VSession     `json:"t2vSessions"`
	Clones   []CloneVoice     `json:"cloneVoices"`
	Prompts  []PromptTemplate `json:"prompts"`
	LogList  []LogEntry       `json:"logs"`
	Config   Settings         `json:"settings"`
	Seeded   bool             `json:"promptsSeeded"`
}

// Store — JSON file store, an toàn goroutine.
type Store struct {
	mu                 sync.RWMutex
	d                  db
	path               string
	DataDir            string
	lastPersistenceErr string
}

// Open mở (hoặc tạo) store tại <dataDir>/db.json và chuẩn bị cây thư mục.
func Open(dataDir string) (*Store, error) {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"", "projects", "downloads", "tmp"} {
		if err := os.MkdirAll(filepath.Join(abs, sub), 0o755); err != nil {
			return nil, err
		}
	}
	s := &Store{path: filepath.Join(abs, "db.json"), DataDir: abs}
	if b, err := os.ReadFile(s.path); err == nil {
		if err := secureFilePermissions(s.path); err != nil {
			return nil, fmt.Errorf("giới hạn quyền db.json: %w", err)
		}
		if err := json.Unmarshal(b, &s.d); err != nil {
			backup, backupErr := os.ReadFile(s.path + ".bak")
			if backupErr != nil || json.Unmarshal(backup, &s.d) != nil {
				return nil, fmt.Errorf("db.json hỏng và không có backup dùng được: %w", err)
			}
			corrupt := s.path + ".corrupt-" + time.Now().Format("20060102-150405")
			if renameErr := os.Rename(s.path, corrupt); renameErr != nil {
				return nil, fmt.Errorf("khôi phục backup nhưng không giữ được db hỏng: %w", renameErr)
			}
			log.Printf("db.json hỏng — đã khôi phục db.json.bak; bản hỏng giữ tại %s", corrupt)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("không đọc được db.json: %w", err)
	}
	s.applyDefaults()
	s.seedPrompts()
	return s, s.saveLocked()
}

func (s *Store) applyDefaults() {
	c := &s.d.Config
	if c.GeminiBase == "" {
		c.GeminiBase = "https://generativelanguage.googleapis.com"
	}
	if c.GeminiModel == "" {
		c.GeminiModel = "gemini-2.5-flash"
	}
	if c.ClaudeBin == "" {
		c.ClaudeBin = "claude"
	}
	if c.YtdlpBin == "" {
		c.YtdlpBin = "yt-dlp"
	}
	if c.DownloadDir == "" {
		c.DownloadDir = filepath.Join(s.DataDir, "downloads")
	}
	if c.Quality == "" {
		c.Quality = "best"
	}
	if c.Threads == 0 {
		c.Threads = 3
	}
	if c.Theme == "" {
		c.Theme = "light"
	}
	if c.UIScale == 0 {
		c.UIScale = 100
	}
	if c.PerfMode == "" {
		c.PerfMode = "auto"
	}
	if c.WhisperModel == "" {
		c.WhisperModel = "small"
	}
	if c.WhisperCompute == "" {
		c.WhisperCompute = "auto"
	}
	if c.OpenAIBase == "" {
		c.OpenAIBase = "https://api.openai.com/v1"
	}
	if c.OpenAIModel == "" {
		c.OpenAIModel = "gpt-4o-mini"
	}
}

// save ghi xuống disk (atomic). Gọi khi ĐANG giữ write lock.
func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.d, "", " ")
	if err != nil {
		return err
	}
	if current, err := os.ReadFile(s.path); err == nil {
		if err := writeSynced(s.path+".bak", current); err != nil {
			return fmt.Errorf("ghi backup db: %w", err)
		}
	}
	tmp := s.path + ".tmp"
	if err := writeSynced(tmp, b); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	if err := secureFilePermissions(s.path); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(s.path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func writeSynced(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return secureFilePermissions(path)
}

// write chạy fn trong write lock rồi persist.
func (s *Store) write(fn func(*db)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, snapshotErr := json.Marshal(s.d)
	fn(&s.d)
	if err := s.saveLocked(); err != nil {
		if snapshotErr == nil {
			_ = json.Unmarshal(before, &s.d)
		}
		s.lastPersistenceErr = err.Error()
		log.Printf("LỖI LƯU DỮ LIỆU: %v", err)
	} else {
		s.lastPersistenceErr = ""
	}
}

func (s *Store) PersistenceError() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastPersistenceErr
}

// NewID sinh ID dạng <prefix>_<8 hex>.
func (s *Store) NewID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func now() time.Time { return time.Now() }
