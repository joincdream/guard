package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"guard/internal/core"
)

// Manager manages session JSON files in a dedicated directory with atomic writes and GC.
type Manager struct {
	dir     string
	isAlive func(int) bool
}

// DefaultDir returns the default session directory (~/.guard/sessions).
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "guard-sessions")
	}
	return filepath.Join(home, ".guard", "sessions")
}

// NewManager creates a Manager backed by dir. If dir is empty, DefaultDir() is used.
func NewManager(dir string) *Manager {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultDir()
	}
	return &Manager{
		dir:     dir,
		isAlive: IsProcessAlive,
	}
}

// Dir returns the directory path where session files are stored.
func (m *Manager) Dir() string {
	return m.dir
}

// ShortID returns up to the first 8 characters of a session ID for compact display.
func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// SessionPath returns the file path for the given session ID.
func (m *Manager) SessionPath(id string) string {
	return filepath.Join(m.dir, sanitizeID(id)+".json")
}

func (m *Manager) lockPath(id string) string {
	return filepath.Join(m.dir, "."+sanitizeID(id)+".lock")
}

func (m *Manager) withSessionLock(id string, fn func() error) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("session ID is required")
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	f, err := os.OpenFile(m.lockPath(id), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open session lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return fmt.Errorf("acquire session lock: %w", err)
	}
	defer func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}()
	return fn()
}

// Create initializes timestamps/mode defaults and atomically persists a new session.
func (m *Manager) Create(sess *Session) error {
	if sess == nil || strings.TrimSpace(sess.ID) == "" {
		return errors.New("session ID is required")
	}
	now := time.Now()
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	sess.UpdatedAt = now
	sess.Mode = normalizeMode(sess.Mode)
	if sess.Events == nil {
		sess.Events = []Event{}
	}
	return m.Save(sess)
}

// Save atomically writes the session JSON file using a temporary file and os.Rename.
func (m *Manager) Save(sess *Session) error {
	if sess == nil || strings.TrimSpace(sess.ID) == "" {
		return errors.New("session ID is required")
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	data = append(data, '\n')

	tmpFile, err := os.CreateTemp(m.dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp session file: %w", err)
	}
	tmpPath := tmpFile.Name()

	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp session file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp session file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp session file: %w", err)
	}

	targetPath := m.SessionPath(sess.ID)
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("atomic rename session file: %w", err)
	}
	cleanupTmp = false
	return nil
}

// Get reads and parses a single session by ID.
func (m *Manager) Get(id string) (*Session, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("session ID is required")
	}
	data, err := os.ReadFile(m.SessionPath(id))
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("unmarshal session %s: %w", id, err)
	}
	if sess.Events == nil {
		sess.Events = []Event{}
	}
	return &sess, nil
}

// Delete removes a session file and its lock file by ID. Missing files are treated as already deleted.
func (m *Manager) Delete(id string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	err := os.Remove(m.SessionPath(id))
	_ = os.Remove(m.lockPath(id))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DeleteAll removes all session JSON files (and associated lock files) from the directory and returns the count removed.
func (m *Manager) DeleteAll() (int, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".lock") {
			_ = os.Remove(filepath.Join(m.dir, name))
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(m.dir, name)); err == nil {
			removed++
		}
	}
	return removed, nil
}

// List reads all valid session JSON files in the directory sorted by UpdatedAt descending.
// Corrupted JSON files are skipped so a single broken file never breaks the aggregator.
func (m *Manager) List() ([]*Session, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, err
	}

	sessions := make([]*Session, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, entry.Name()))
		if err != nil {
			continue
		}
		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}
		if sess.Events == nil {
			sess.Events = []Event{}
		}
		sessions = append(sessions, &sess)
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].UpdatedAt.Equal(sessions[j].UpdatedAt) {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// UpdateMode atomically updates the mode of an existing session under an exclusive file lock.
func (m *Manager) UpdateMode(id string, mode string) (*Session, error) {
	var updated *Session
	err := m.withSessionLock(id, func() error {
		sess, err := m.Get(id)
		if err != nil {
			return err
		}
		sess.Mode = normalizeMode(mode)
		sess.UpdatedAt = time.Now()
		if err := m.Save(sess); err != nil {
			return err
		}
		updated = sess
		return nil
	})
	return updated, err
}

// ResetViolations resets ViolationCount to 0 for an existing session under an exclusive file lock.
func (m *Manager) ResetViolations(id string) (*Session, error) {
	var updated *Session
	err := m.withSessionLock(id, func() error {
		sess, err := m.Get(id)
		if err != nil {
			return err
		}
		sess.ViolationCount = 0
		sess.UpdatedAt = time.Now()
		if err := m.Save(sess); err != nil {
			return err
		}
		updated = sess
		return nil
	})
	return updated, err
}

// AppendEvent appends a tool evaluation event to the session, rotates Events to at most MaxEvents (50),
// increments ViolationCount on "deny", and atomically persists the updated session under an exclusive file lock.
func (m *Manager) AppendEvent(id string, ev Event) (*Session, error) {
	var updated *Session
	err := m.withSessionLock(id, func() error {
		sess, err := m.Get(id)
		if err != nil {
			return err
		}
		if ev.Timestamp.IsZero() {
			ev.Timestamp = time.Now()
		}
		ev.Decision = strings.ToLower(strings.TrimSpace(ev.Decision))

		sess.Events = append(sess.Events, ev)
		if len(sess.Events) > MaxEvents {
			sess.Events = sess.Events[len(sess.Events)-MaxEvents:]
		}
		if ev.Decision == "deny" {
			sess.ViolationCount++
		}
		sess.UpdatedAt = time.Now()

		if err := m.Save(sess); err != nil {
			return err
		}
		updated = sess
		return nil
	})
	return updated, err
}

func normalizeMode(mode string) string {
	m := core.NormalizeMode(mode)
	if m == "" {
		return core.ModeDev
	}
	return m
}

func sanitizeID(id string) string {
	orig := strings.TrimSpace(id)
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_")
	s := replacer.Replace(orig)
	for strings.HasPrefix(s, ".") {
		s = "_" + strings.TrimPrefix(s, ".")
	}
	if s != orig {
		sum := sha256.Sum256([]byte(orig))
		s = s + "_" + hex.EncodeToString(sum[:4])
	}
	return s
}
