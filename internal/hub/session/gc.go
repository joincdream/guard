package session

import (
	"errors"
	"os"
	"syscall"
)

// IsProcessAlive checks whether a process with the given PID is currently alive
// using POSIX signal 0 (process.Signal(syscall.Signal(0))).
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists, even though we lack permission to signal it.
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}

// CleanAndList scans all persisted sessions once, removes any session whose bound PID is non-positive
// or no longer alive, and returns the surviving active sessions along with the removed session IDs.
func (m *Manager) CleanAndList() ([]*Session, []string, error) {
	sessions, err := m.List()
	if err != nil {
		return nil, nil, err
	}

	aliveFn := m.isAlive
	if aliveFn == nil {
		aliveFn = IsProcessAlive
	}

	active := make([]*Session, 0, len(sessions))
	var removed []string
	for _, sess := range sessions {
		if sess.PID <= 0 || !aliveFn(sess.PID) {
			if err := m.Delete(sess.ID); err == nil {
				removed = append(removed, sess.ID)
			}
			continue
		}
		active = append(active, sess)
	}
	return active, removed, nil
}

// CleanStaleSessions scans all persisted sessions and removes any session whose bound PID is no longer alive,
// or whose PID is non-positive (unbound/invalid).
// Returns the list of removed session IDs.
func (m *Manager) CleanStaleSessions() ([]string, error) {
	_, removed, err := m.CleanAndList()
	return removed, err
}
