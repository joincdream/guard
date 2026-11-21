package handoff

import (
	"fmt"
	"strings"
)

// FormatEnvelope renders a HandoffItem into the standard Markdown Envelope
// written to ~/.guard/sessions/<sessionID>.inbox.
func FormatEnvelope(item HandoffItem, scopeLocked bool) (string, error) {
	if strings.TrimSpace(item.ID) == "" {
		return "", fmt.Errorf("%w: envelope requires non-empty id", ErrInvalidItem)
	}
	if !ValidType(item.Type) {
		return "", fmt.Errorf("%w: envelope requires valid type, got %q", ErrInvalidItem, item.Type)
	}
	if strings.TrimSpace(item.Summary) == "" {
		return "", fmt.Errorf("%w: envelope requires non-empty summary", ErrInvalidItem)
	}

	var b strings.Builder

	// Opening envelope marker with metadata
	if item.ParentID != "" {
		fmt.Fprintf(&b, "<!-- GUARD_HANDOFF_ENVELOPE: START (ID: %s, Type: %s, Parent: %s) -->\n",
			item.ID, item.Type, item.ParentID)
	} else {
		fmt.Fprintf(&b, "<!-- GUARD_HANDOFF_ENVELOPE: START (ID: %s, Type: %s) -->\n",
			item.ID, item.Type)
	}

	// Header line
	senderLabel := formatSenderLabel(item.From)
	fmt.Fprintf(&b, "### 🔔 [Handoff: %s] from %s\n\n", strings.ToUpper(string(item.Type)), senderLabel)

	// Summary
	fmt.Fprintf(&b, "**요약**: %s\n", strings.TrimSpace(item.Summary))

	// References (Read-Only context)
	if len(item.References) > 0 {
		b.WriteString("\n**참조 문서 (Read-Only)**:\n")
		for _, ref := range item.References {
			trimmed := strings.TrimSpace(ref)
			if strings.HasPrefix(trimmed, "@") {
				fmt.Fprintf(&b, "- %s\n", trimmed)
			} else {
				fmt.Fprintf(&b, "- @%s\n", trimmed)
			}
		}
	}

	// Targets
	if len(item.Targets) > 0 {
		if scopeLocked {
			b.WriteString("\n**수정 허용 대상 (Scope Locked)**:\n")
		} else {
			b.WriteString("\n**작업 대상**:\n")
		}
		for _, target := range item.Targets {
			fmt.Fprintf(&b, "- `%s`\n", strings.TrimSpace(target))
		}
	}

	// Optional inline payload
	if payload := strings.TrimSpace(item.Payload); payload != "" {
		b.WriteString("\n")
		b.WriteString(payload)
		b.WriteString("\n")
	}

	// Completion instruction & closing marker
	fmt.Fprintf(&b, "\n> 작업 완료 후 `guard handoff ack %s`을 실행하세요.\n", item.ID)
	b.WriteString("<!-- GUARD_HANDOFF_ENVELOPE: END -->\n")

	return b.String(), nil
}

func formatSenderLabel(from HandoffNode) string {
	mode := strings.ToUpper(strings.TrimSpace(from.Mode))
	sessID := strings.TrimSpace(from.SessionID)
	switch {
	case mode != "" && sessID != "":
		return fmt.Sprintf("%s (%s)", mode, sessID)
	case mode != "":
		return mode
	case sessID != "":
		return sessID
	default:
		return "UNKNOWN"
	}
}
