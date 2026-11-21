package handoff

import (
	"errors"
	"fmt"
	"strings"

	"guard/internal/hub/session"
)

// ApproveResult describes the outcome of approving and routing a handoff ticket.
type ApproveResult struct {
	Item        HandoffItem
	SessionID   string
	InboxPath   string
	ScopeLocked bool
	Buffered    bool
}

// ApproveAndRoute transitions a pending handoff ticket to approved and routes it:
//   - If a target session is active (matched by explicit SessionID or by Mode+workspace),
//     it binds conditional Scope Lock (when applicable), delivers the Markdown Envelope
//     to the session inbox, and records To.SessionID atomically in queue.json.
//   - If targeting a Mode with no currently active session, it keeps To.SessionID empty
//     with status approved so the next launcher startup claims and delivers it.
func ApproveAndRoute(handoffMgr *Manager, sessMgr *session.Manager, workspaceRoot string, id string) (ApproveResult, error) {
	if handoffMgr == nil {
		return ApproveResult{}, errors.New("handoff manager is required")
	}
	if sessMgr == nil {
		return ApproveResult{}, errors.New("session manager is required")
	}
	if !ValidSafeID(id) {
		return ApproveResult{}, fmt.Errorf("%w: invalid id %q", ErrInvalidItem, id)
	}

	var result ApproveResult
	err := withFileLock(handoffMgr.queueLockPath(), func() error {
		items, err := handoffMgr.loadQueueUnlocked()
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

		it := items[idx]
		if err := ValidateStatusTransition(it.Status, StatusApproved); err != nil {
			return err
		}

		var targetSess *session.Session
		if it.To.SessionID != "" {
			sess, err := sessMgr.Get(it.To.SessionID)
			if err != nil {
				return fmt.Errorf("resolve target session %s: %w", it.To.SessionID, err)
			}
			targetSess = sess
		} else {
			sess, err := sessMgr.FindActiveByMode(it.To.Mode, workspaceRoot)
			if err != nil {
				if !errors.Is(err, session.ErrNoActiveSession) {
					return fmt.Errorf("find active session for mode %s: %w", it.To.Mode, err)
				}
				// No active session for this mode: buffer in queue as approved.
				it.Status = StatusApproved
				it.UpdatedAt = handoffMgr.now().UTC()
				items[idx] = it
				if err := handoffMgr.saveQueueUnlocked(items); err != nil {
					return err
				}
				result = ApproveResult{
					Item:     it,
					Buffered: true,
				}
				return nil
			}
			targetSess = sess
		}

		// Active session resolved: apply conditional Scope Lock and deliver to inbox.
		scopeLocked := ShouldApplyScopeLock(it, targetSess.Mode)
		if scopeLocked {
			if _, err := sessMgr.UpdateScope(targetSess.ID, it.Targets); err != nil {
				return fmt.Errorf("apply scope lock to session %s: %w", targetSess.ID, err)
			}
		}

		it.Status = StatusApproved
		it.To.SessionID = targetSess.ID
		if strings.TrimSpace(it.To.Mode) == "" {
			it.To.Mode = strings.ToLower(strings.TrimSpace(targetSess.Mode))
		}
		it.UpdatedAt = handoffMgr.now().UTC()

		inboxPath, err := handoffMgr.DeliverToInbox(targetSess.ID, it, scopeLocked)
		if err != nil {
			return fmt.Errorf("deliver handoff to session %s: %w", targetSess.ID, err)
		}

		items[idx] = it
		if err := handoffMgr.saveQueueUnlocked(items); err != nil {
			return err
		}

		result = ApproveResult{
			Item:        it,
			SessionID:   targetSess.ID,
			InboxPath:   inboxPath,
			ScopeLocked: scopeLocked,
			Buffered:    false,
		}
		return nil
	})
	if err != nil {
		return ApproveResult{}, err
	}
	return result, nil
}
