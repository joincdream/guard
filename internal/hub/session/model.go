package session

import "time"

// MaxEvents is the maximum number of recent tool-call events retained per session.
const MaxEvents = 50

// Event records a single guardrail evaluation result for a tool invocation.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Decision  string    `json:"decision"` // "allow" or "deny"
	Tool      string    `json:"tool"`
	Target    string    `json:"target"`
	RuleName  string    `json:"ruleName,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// Session represents the persisted state of an active agent session stored under ~/.guard/sessions/<id>.json.
type Session struct {
	ID                string    `json:"id"`
	Mode              string    `json:"mode"` // DEV, TEST, PLAN, ALL
	PID               int       `json:"pid"`
	Workspace         string    `json:"workspace"`
	AllowedWritePaths []string  `json:"allowedWritePaths,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	ViolationCount    int       `json:"violationCount"`
	Events            []Event   `json:"events"`
}
