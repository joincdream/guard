package handoff

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// HandoffType classifies the intent of a handoff ticket.
type HandoffType string

const (
	TypeTask   HandoffType = "task"   // New implementation or task delegation (e.g. PLAN -> DEV)
	TypeVerify HandoffType = "verify" // Verification request (e.g. DEV -> TEST)
	TypeFix    HandoffType = "fix"    // Defect fix request (e.g. TEST -> DEV)
	TypeReview HandoffType = "review" // Design review or inquiry (e.g. DEV -> PLAN)
)

// HandoffStatus represents the lifecycle state of a handoff ticket.
type HandoffStatus string

const (
	StatusPending  HandoffStatus = "pending"
	StatusApproved HandoffStatus = "approved"
	StatusRejected HandoffStatus = "rejected"
	StatusAcked    HandoffStatus = "acked"
)

// Normalized mode values for HandoffNode.
const (
	ModePlan = "plan"
	ModeDev  = "dev"
	ModeTest = "test"
	ModeAll  = "all"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Sentinel errors returned by handoff validation and queue operations.
var (
	ErrNotFound          = errors.New("handoff item not found")
	ErrInvalidItem       = errors.New("invalid handoff item")
	ErrInvalidTransition = errors.New("invalid handoff status transition")
	ErrInvalidSessionID  = errors.New("invalid session ID")
)

// HandoffNode identifies the sender or receiver of a handoff ticket.
type HandoffNode struct {
	Mode      string `json:"mode,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// HandoffItem is the canonical envelope persisted in the handoff queue.
type HandoffItem struct {
	ID           string        `json:"id"`
	ParentID     string        `json:"parent_id,omitempty"`
	Type         HandoffType   `json:"type"`
	From         HandoffNode   `json:"from"`
	To           HandoffNode   `json:"to"`
	Summary      string        `json:"summary"`
	References   []string      `json:"references,omitempty"`
	Targets      []string      `json:"targets,omitempty"`
	Payload      string        `json:"payload,omitempty"`
	Status       HandoffStatus `json:"status"`
	RejectReason string        `json:"reject_reason,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// ValidType reports whether t is a supported handoff type.
func ValidType(t HandoffType) bool {
	switch t {
	case TypeTask, TypeVerify, TypeFix, TypeReview:
		return true
	default:
		return false
	}
}

// ValidStatus reports whether s is a supported handoff status.
func ValidStatus(s HandoffStatus) bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected, StatusAcked:
		return true
	default:
		return false
	}
}

// ValidAgentMode reports whether m is a supported agent session mode.
func ValidAgentMode(m string) bool {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case ModePlan, ModeDev, ModeTest, ModeAll:
		return true
	default:
		return false
	}
}

// ValidSafeID reports whether id is a safe identifier without path traversal characters.
func ValidSafeID(id string) bool {
	if id == "" || id == "." || id == ".." || strings.Contains(id, "..") {
		return false
	}
	return safeIDPattern.MatchString(id)
}

// ParseTarget parses a CLI target string ("plan", "dev", "test", "all", or "<sessionId>").
func ParseTarget(raw string) (HandoffNode, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return HandoffNode{}, fmt.Errorf("%w: target is required", ErrInvalidItem)
	}

	lower := strings.ToLower(trimmed)
	if ValidAgentMode(lower) {
		return HandoffNode{
			Mode: lower,
		}, nil
	}

	if !ValidSafeID(trimmed) {
		return HandoffNode{}, fmt.Errorf("%w: invalid target session ID %q", ErrInvalidItem, trimmed)
	}
	return HandoffNode{
		SessionID: trimmed,
	}, nil
}

// ValidateAsTarget checks that the node represents a valid routing destination.
func (n HandoffNode) ValidateAsTarget() error {
	mode := strings.ToLower(strings.TrimSpace(n.Mode))
	if mode == "" && n.SessionID == "" {
		return fmt.Errorf("%w: target must specify mode or session_id", ErrInvalidItem)
	}
	if mode != "" && !ValidAgentMode(mode) {
		return fmt.Errorf("%w: unsupported target mode %q", ErrInvalidItem, n.Mode)
	}
	if n.SessionID != "" && !ValidSafeID(n.SessionID) {
		return fmt.Errorf("%w: invalid target session_id %q", ErrInvalidItem, n.SessionID)
	}
	return nil
}

// ValidateAsSender checks that the sender node fields, if present, are well-formed.
func (n HandoffNode) ValidateAsSender() error {
	mode := strings.ToLower(strings.TrimSpace(n.Mode))
	if mode != "" && !ValidAgentMode(mode) {
		return fmt.Errorf("%w: unsupported sender mode %q", ErrInvalidItem, n.Mode)
	}
	if n.SessionID != "" && !ValidSafeID(n.SessionID) {
		return fmt.Errorf("%w: invalid sender session_id %q", ErrInvalidItem, n.SessionID)
	}
	return nil
}

// Validate verifies all invariants of a HandoffItem before persistence.
func (item HandoffItem) Validate() error {
	if item.ID != "" && !ValidSafeID(item.ID) {
		return fmt.Errorf("%w: invalid id %q", ErrInvalidItem, item.ID)
	}
	if item.ParentID != "" && !ValidSafeID(item.ParentID) {
		return fmt.Errorf("%w: invalid parent_id %q", ErrInvalidItem, item.ParentID)
	}
	if !ValidType(item.Type) {
		return fmt.Errorf("%w: unsupported type %q", ErrInvalidItem, item.Type)
	}
	if !ValidStatus(item.Status) {
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidItem, item.Status)
	}
	if err := item.From.ValidateAsSender(); err != nil {
		return err
	}
	if err := item.To.ValidateAsTarget(); err != nil {
		return err
	}

	if strings.TrimSpace(item.Summary) == "" {
		return fmt.Errorf("%w: summary is required", ErrInvalidItem)
	}
	for i, ref := range item.References {
		if strings.TrimSpace(ref) == "" {
			return fmt.Errorf("%w: references[%d] must not be empty", ErrInvalidItem, i)
		}
	}
	for i, target := range item.Targets {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("%w: targets[%d] must not be empty", ErrInvalidItem, i)
		}
	}
	return nil
}

// ValidateStatusTransition enforces valid lifecycle state changes.
func ValidateStatusTransition(from, to HandoffStatus) error {
	if !ValidStatus(from) || !ValidStatus(to) {
		return fmt.Errorf("%w: %q -> %q", ErrInvalidTransition, from, to)
	}
	switch from {
	case StatusPending:
		if to == StatusApproved || to == StatusRejected {
			return nil
		}
	case StatusApproved:
		if to == StatusAcked {
			return nil
		}
	}
	return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, from, to)
}

// GenerateID creates a collision-resistant handoff ticket identifier (e.g. hnd-20261121-a1b2c3d4).
func GenerateID(now time.Time) (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate handoff id: %w", err)
	}
	return fmt.Sprintf("hnd-%s-%s", now.UTC().Format("20060102"), hex.EncodeToString(buf[:])), nil
}

// ShouldApplyScopeLock reports whether approving and delivering item to targetMode
// should automatically bind AllowedWritePaths on the target session.
func ShouldApplyScopeLock(item HandoffItem, targetMode string) bool {
	if item.Type != TypeFix && item.Type != TypeTask {
		return false
	}
	if len(item.Targets) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(targetMode)) {
	case ModeDev, ModeAll:
		return true
	default:
		return false
	}
}
