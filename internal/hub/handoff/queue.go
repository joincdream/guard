package handoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// DefaultMaxQueueItems bounds the number of retained tickets in queue.json.
// Active tickets (pending/approved) are never evicted during compaction.
const DefaultMaxQueueItems = 200

// ListFilter narrows results returned by Manager.List.
type ListFilter struct {
	Status      HandoffStatus
	ToMode      string
	ToSessionID string
	OnlyActive  bool // Matches pending or approved tickets
}

// Manager provides concurrency-safe file storage for the handoff queue and session inboxes.
type Manager struct {
	handoffDir string
	sessionDir string
	maxItems   int
	now        func() time.Time
}

// DefaultHandoffDir returns ~/.guard/handoff.
func DefaultHandoffDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("resolve user home dir: empty path")
	}
	return filepath.Join(home, ".guard", "handoff"), nil
}

// DefaultSessionDir returns ~/.guard/sessions.
func DefaultSessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("resolve user home dir: empty path")
	}
	return filepath.Join(home, ".guard", "sessions"), nil
}

// NewManager creates a Manager backed by handoffDir and sessionDir.
// If either directory is empty, its corresponding default directory under ~/.guard is used.
func NewManager(handoffDir, sessionDir string) (*Manager, error) {
	var err error
	if strings.TrimSpace(handoffDir) == "" {
		handoffDir, err = DefaultHandoffDir()
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(sessionDir) == "" {
		sessionDir, err = DefaultSessionDir()
		if err != nil {
			return nil, err
		}
	}
	return &Manager{
		handoffDir: handoffDir,
		sessionDir: sessionDir,
		maxItems:   DefaultMaxQueueItems,
		now:        time.Now,
	}, nil
}

// QueuePath returns the path to queue.json.
func (m *Manager) QueuePath() string {
	return filepath.Join(m.handoffDir, "queue.json")
}

// InboxPath returns the path to a session's inbox file.
func (m *Manager) InboxPath(sessionID string) (string, error) {
	if !ValidSafeID(sessionID) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSessionID, sessionID)
	}
	return filepath.Join(m.sessionDir, sessionID+".inbox"), nil
}

func (m *Manager) queueLockPath() string {
	return filepath.Join(m.handoffDir, ".queue.lock")
}

func (m *Manager) inboxLockPath(sessionID string) string {
	return filepath.Join(m.sessionDir, "."+sessionID+".inbox.lock")
}

func withFileLock(lockPath string, fn func() error) (retErr error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return fmt.Errorf("acquire file lock: %w", err)
	}
	defer func() {
		unlockErr := syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		closeErr := f.Close()
		if retErr == nil {
			if unlockErr != nil {
				retErr = fmt.Errorf("release file lock: %w", unlockErr)
			} else if closeErr != nil {
				retErr = fmt.Errorf("close lock file: %w", closeErr)
			}
		}
	}()
	return fn()
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	cleanup = false
	return nil
}

func (m *Manager) loadQueueUnlocked() ([]HandoffItem, error) {
	data, err := os.ReadFile(m.QueuePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []HandoffItem{}, nil
		}
		return nil, fmt.Errorf("read handoff queue: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return []HandoffItem{}, nil
	}
	var items []HandoffItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse handoff queue: %w", err)
	}
	return items, nil
}

func (m *Manager) saveQueueUnlocked(items []HandoffItem) error {
	compacted := compactQueue(items, m.maxItems)
	data, err := json.MarshalIndent(compacted, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal handoff queue: %w", err)
	}
	data = append(data, '\n')
	return writeFileAtomic(m.QueuePath(), data, 0o600)
}

func compactQueue(items []HandoffItem, maxItems int) []HandoffItem {
	if maxItems <= 0 || len(items) <= maxItems {
		return items
	}
	excess := len(items) - maxItems
	out := make([]HandoffItem, 0, len(items))
	for _, it := range items {
		isCompleted := it.Status == StatusAcked || it.Status == StatusRejected
		if excess > 0 && isCompleted {
			excess--
			continue
		}
		out = append(out, it)
	}
	return out
}

// Enqueue validates and appends a new pending handoff ticket to queue.json.
func (m *Manager) Enqueue(item HandoffItem) (HandoffItem, error) {
	now := m.now().UTC()
	if strings.TrimSpace(item.ID) == "" {
		id, err := GenerateID(now)
		if err != nil {
			return HandoffItem{}, err
		}
		item.ID = id
	}
	item.From.Mode = strings.ToLower(strings.TrimSpace(item.From.Mode))
	item.To.Mode = strings.ToLower(strings.TrimSpace(item.To.Mode))
	item.Summary = strings.TrimSpace(item.Summary)
	item.Status = StatusPending
	item.CreatedAt = now
	item.UpdatedAt = now

	if err := item.Validate(); err != nil {
		return HandoffItem{}, err
	}

	err := withFileLock(m.queueLockPath(), func() error {
		items, err := m.loadQueueUnlocked()
		if err != nil {
			return err
		}
		for _, existing := range items {
			if existing.ID == item.ID {
				return fmt.Errorf("%w: duplicate handoff id %q", ErrInvalidItem, item.ID)
			}
		}
		items = append(items, item)
		return m.saveQueueUnlocked(items)
	})
	if err != nil {
		return HandoffItem{}, err
	}
	return item, nil
}

// List returns handoff tickets matching filter.
func (m *Manager) List(filter ListFilter) ([]HandoffItem, error) {
	var matched []HandoffItem
	err := withFileLock(m.queueLockPath(), func() error {
		items, err := m.loadQueueUnlocked()
		if err != nil {
			return err
		}
		toMode := strings.ToLower(strings.TrimSpace(filter.ToMode))
		toSession := strings.TrimSpace(filter.ToSessionID)

		matched = make([]HandoffItem, 0, len(items))
		for _, it := range items {
			if filter.OnlyActive && it.Status != StatusPending && it.Status != StatusApproved {
				continue
			}
			if filter.Status != "" && it.Status != filter.Status {
				continue
			}
			if toMode != "" && !strings.EqualFold(it.To.Mode, toMode) {
				continue
			}
			if toSession != "" && it.To.SessionID != toSession {
				continue
			}
			matched = append(matched, it)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matched, nil
}

// Get retrieves a single handoff ticket by ID.
func (m *Manager) Get(id string) (*HandoffItem, error) {
	if !ValidSafeID(id) {
		return nil, fmt.Errorf("%w: invalid id %q", ErrInvalidItem, id)
	}
	var found *HandoffItem
	err := withFileLock(m.queueLockPath(), func() error {
		items, err := m.loadQueueUnlocked()
		if err != nil {
			return err
		}
		for i := range items {
			if items[i].ID == id {
				cp := items[i]
				found = &cp
				return nil
			}
		}
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// UpdateStatus transitions a ticket to status after validating the state machine.
func (m *Manager) UpdateStatus(id string, status HandoffStatus, reason string) (*HandoffItem, error) {
	if !ValidSafeID(id) {
		return nil, fmt.Errorf("%w: invalid id %q", ErrInvalidItem, id)
	}
	var updated *HandoffItem
	err := withFileLock(m.queueLockPath(), func() error {
		items, err := m.loadQueueUnlocked()
		if err != nil {
			return err
		}
		idx := -1
		for i := range items {
			if items[i].ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		if err := ValidateStatusTransition(items[idx].Status, status); err != nil {
			return err
		}
		items[idx].Status = status
		items[idx].UpdatedAt = m.now().UTC()
		if status == StatusRejected {
			items[idx].RejectReason = strings.TrimSpace(reason)
		}
		if err := m.saveQueueUnlocked(items); err != nil {
			return err
		}
		cp := items[idx]
		updated = &cp
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// DeliverToInbox formats item as a Markdown Envelope and atomically appends it
// to ~/.guard/sessions/<sessionID>.inbox.
func (m *Manager) DeliverToInbox(sessionID string, item HandoffItem, scopeLocked bool) (string, error) {
	inboxPath, err := m.InboxPath(sessionID)
	if err != nil {
		return "", err
	}
	envelope, err := FormatEnvelope(item, scopeLocked)
	if err != nil {
		return "", err
	}

	err = withFileLock(m.inboxLockPath(sessionID), func() error {
		existing, readErr := os.ReadFile(inboxPath)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("read session inbox: %w", readErr)
		}

		var b strings.Builder
		if len(existing) > 0 {
			b.Write(existing)
			if !strings.HasSuffix(string(existing), "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		b.WriteString(envelope)

		return writeFileAtomic(inboxPath, []byte(b.String()), 0o600)
	})
	if err != nil {
		return "", err
	}
	return inboxPath, nil
}

// ConsumePendingForMode atomically claims all approved, unassigned tickets targeting mode,
// assigns them to sessionID, and delivers them to the session's inbox.
func (m *Manager) ConsumePendingForMode(mode, sessionID string) ([]HandoffItem, error) {
	normMode := strings.ToLower(strings.TrimSpace(mode))
	if !ValidAgentMode(normMode) {
		return nil, fmt.Errorf("%w: unsupported mode %q", ErrInvalidItem, mode)
	}
	if !ValidSafeID(sessionID) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidSessionID, sessionID)
	}

	var claimed []HandoffItem
	err := withFileLock(m.queueLockPath(), func() error {
		items, err := m.loadQueueUnlocked()
		if err != nil {
			return err
		}

		now := m.now().UTC()
		modified := false
		for i := range items {
			it := items[i]
			if it.Status != StatusApproved {
				continue
			}
			if it.To.SessionID != "" {
				continue
			}
			if !strings.EqualFold(it.To.Mode, normMode) {
				continue
			}

			it.To.SessionID = sessionID
			it.UpdatedAt = now
			scopeLocked := ShouldApplyScopeLock(it, normMode)
			if _, err := m.DeliverToInbox(sessionID, it, scopeLocked); err != nil {
				return fmt.Errorf("deliver buffered handoff %s: %w", it.ID, err)
			}
			items[i] = it
			claimed = append(claimed, it)
			modified = true
		}

		if !modified {
			return nil
		}
		return m.saveQueueUnlocked(items)
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}
