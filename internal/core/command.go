package core

import (
	"path"
	"strings"
)

var fileViewers = map[string]bool{
	"cat":     true,
	"head":    true,
	"tail":    true,
	"more":    true,
	"less":    true,
	"strings": true,
	"nl":      true,
	"tac":     true,
	"bat":     true,
}

var inlineScriptRunners = map[string]bool{
	"python":  true,
	"python3": true,
	"node":    true,
	"ruby":    true,
	"perl":    true,
	"bash":    true,
	"sh":      true,
	"zsh":     true,
}

// evaluateCommandRules parses a shell command line into segments and inspects each segment
// for read/write policy violations and shell bypass attempts.
func evaluateCommandRules(policy *Policy, currentMode string, cmdLine string) (string, string) {
	segments := splitCommandSegments(cmdLine)
	if len(segments) == 0 {
		return "", ""
	}

	hasReadRules := hasRulesFor(policy, currentMode, "read")
	hasWriteRules := hasRulesFor(policy, currentMode, "write")

	for _, seg := range segments {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" {
			continue
		}

		tokens := tokenizeSegment(trimmed)
		if len(tokens) == 0 {
			continue
		}

		if isSafeSegment(tokens) {
			continue
		}

		// Check read bypass attempts (e.g. DEV mode test-file read, TEST mode prod-code read).
		if hasReadRules {
			if ruleName, reason := inspectReadSegment(policy, currentMode, trimmed, tokens); reason != "" {
				return ruleName, reason
			}
		}

		// Check write bypass attempts (e.g. PLAN mode source code write).
		if hasWriteRules {
			if ruleName, reason := inspectWriteSegment(policy, currentMode, trimmed, tokens); reason != "" {
				return ruleName, reason
			}
		}
	}

	return "", ""
}

func hasRulesFor(policy *Policy, currentMode string, actionType string) bool {
	if policy == nil {
		return false
	}
	for _, r := range policy.Rules {
		if (r.Mode == "" || strings.EqualFold(r.Mode, currentMode)) &&
			(r.Type == "" || strings.EqualFold(r.Type, actionType)) {
			return true
		}
	}
	return false
}

// isSafeSegment returns true if a tokenized command segment is a known safe read-only/build command
// with no output redirection.
func isSafeSegment(tokens []string) bool {
	// Any output redirection disqualifies a segment from being automatically safe.
	if hasOutputRedirection(tokens) {
		return false
	}

	cmdIdx := findCommandIndex(tokens)
	if cmdIdx < 0 {
		return false
	}

	cmd := path.Base(tokens[cmdIdx])
	args := tokens[cmdIdx+1:]

	switch cmd {
	case "git":
		if len(args) == 0 {
			return false
		}
		switch args[0] {
		case "status", "branch", "commit":
			return true
		case "log":
			// git log is safe unless it prints diffs/patches.
			for _, a := range args[1:] {
				if a == "-p" || a == "--patch" || a == "-u" || strings.HasPrefix(a, "--patch=") {
					return false
				}
			}
			return true
		}
	case "go":
		if len(args) > 0 && (args[0] == "build" || args[0] == "vet") {
			return true
		}
	}

	return false
}

func inspectReadSegment(policy *Policy, currentMode string, rawSeg string, tokens []string) (string, string) {
	cmdIdx := findCommandIndex(tokens)
	if cmdIdx < 0 {
		return "", ""
	}

	cmd := path.Base(tokens[cmdIdx])
	args := tokens[cmdIdx+1:]

	// Pattern A: File dumpers / viewers (cat, head, tail, more, less, strings, etc.)
	if fileViewers[cmd] {
		for _, arg := range extractPathArgs(args) {
			if ruleName, reason := evaluatePathRules(policy, currentMode, "read", arg); reason != "" {
				return ruleName, reason
			}
		}
	}

	// Input redirection (< target) on any command
	for _, target := range extractInputRedirectionTargets(tokens) {
		if ruleName, reason := evaluatePathRules(policy, currentMode, "read", target); reason != "" {
			return ruleName, reason
		}
	}

	// Pattern B: Git inspectors (git show, git diff, git checkout, git log -p)
	if cmd == "git" && len(args) >= 1 {
		subCmd := args[0]
		isGitInspector := subCmd == "show" || subCmd == "diff" || subCmd == "checkout"
		if subCmd == "log" {
			for _, a := range args[1:] {
				if a == "-p" || a == "--patch" || a == "-u" {
					isGitInspector = true
					break
				}
			}
		}
		if isGitInspector {
			for _, arg := range args[1:] {
				if strings.HasPrefix(arg, "-") {
					continue
				}
				if ruleName, reason := evaluatePathRules(policy, currentMode, "read", arg); reason != "" {
					return ruleName, reason
				}
				// Handle rev:path syntax such as HEAD:pkg/foo_test.go
				if idx := strings.Index(arg, ":"); idx >= 0 && idx+1 < len(arg) {
					revPath := arg[idx+1:]
					if ruleName, reason := evaluatePathRules(policy, currentMode, "read", revPath); reason != "" {
						return ruleName, reason
					}
				}
			}
		}
	}

	// Pattern C: Inline scripting / subshells (python -c, node -e, bash -c, etc.)
	if ruleName, reason := inspectInlineScript(policy, currentMode, "read", cmd, args, rawSeg); reason != "" {
		return ruleName, reason
	}

	return "", ""
}

func inspectWriteSegment(policy *Policy, currentMode string, rawSeg string, tokens []string) (string, string) {
	// 1. Output redirection (>, >>)
	for _, target := range extractOutputRedirectionTargets(tokens) {
		if ruleName, reason := evaluatePathRules(policy, currentMode, "write", target); reason != "" {
			return ruleName, reason
		}
	}

	cmdIdx := findCommandIndex(tokens)
	if cmdIdx < 0 {
		return "", ""
	}

	cmd := path.Base(tokens[cmdIdx])
	args := tokens[cmdIdx+1:]

	// 2. tee
	if cmd == "tee" {
		for _, arg := range extractPathArgs(args) {
			if ruleName, reason := evaluatePathRules(policy, currentMode, "write", arg); reason != "" {
				return ruleName, reason
			}
		}
	}

	// 3. sed -i / sed --in-place
	if cmd == "sed" {
		inPlace := false
		for _, a := range args {
			if a == "-i" || strings.HasPrefix(a, "-i") || a == "--in-place" || strings.HasPrefix(a, "--in-place=") {
				inPlace = true
				break
			}
		}
		if inPlace {
			for _, arg := range extractPathArgs(args) {
				if ruleName, reason := evaluatePathRules(policy, currentMode, "write", arg); reason != "" {
					return ruleName, reason
				}
			}
		}
	}

	// 4. Inline scripting writing to protected paths (e.g. python -c "open('src/internal/a.go','w')...")
	if ruleName, reason := inspectInlineScript(policy, currentMode, "write", cmd, args, rawSeg); reason != "" {
		return ruleName, reason
	}

	return "", ""
}

func inspectInlineScript(policy *Policy, currentMode string, actionType string, cmd string, args []string, rawSeg string) (string, string) {
	if !inlineScriptRunners[cmd] {
		return "", ""
	}
	hasInlineFlag := false
	for _, a := range args {
		if a == "-c" || a == "-e" || a == "--eval" {
			hasInlineFlag = true
			break
		}
	}
	if !hasInlineFlag {
		return "", ""
	}
	for _, candidate := range extractScriptCandidates(rawSeg) {
		if ruleName, reason := evaluatePathRules(policy, currentMode, actionType, candidate); reason != "" {
			return ruleName, reason
		}
	}
	return "", ""
}

// splitCommandSegments splits a shell command line by control operators (|, ||, &&, ;, \n)
// while respecting single and double quotes and backslash escapes.
func splitCommandSegments(cmdLine string) []string {
	var segments []string
	var cur strings.Builder

	inSingle := false
	inDouble := false
	escaped := false

	n := len(cmdLine)
	for i := 0; i < n; i++ {
		ch := cmdLine[i]

		if escaped {
			cur.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			cur.WriteByte(ch)
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			cur.WriteByte(ch)
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			cur.WriteByte(ch)
			continue
		}

		if !inSingle && !inDouble {
			if ch == ';' || ch == '\n' {
				if s := strings.TrimSpace(cur.String()); s != "" {
					segments = append(segments, s)
				}
				cur.Reset()
				continue
			}
			if ch == '|' {
				if i+1 < n && cmdLine[i+1] == '|' {
					i++ // consume '||'
				}
				if s := strings.TrimSpace(cur.String()); s != "" {
					segments = append(segments, s)
				}
				cur.Reset()
				continue
			}
			if ch == '&' && i+1 < n && cmdLine[i+1] == '&' {
				i++ // consume '&&'
				if s := strings.TrimSpace(cur.String()); s != "" {
					segments = append(segments, s)
				}
				cur.Reset()
				continue
			}
		}

		cur.WriteByte(ch)
	}

	if s := strings.TrimSpace(cur.String()); s != "" {
		segments = append(segments, s)
	}
	return segments
}

// tokenizeSegment splits a single command segment into shell tokens, separating redirection operators (> >> <)
// and stripping outer quotes.
func tokenizeSegment(seg string) []string {
	var tokens []string
	var cur strings.Builder

	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	n := len(seg)
	for i := 0; i < n; i++ {
		ch := seg[i]

		if escaped {
			cur.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if !inSingle && !inDouble {
			if ch == ' ' || ch == '\t' || ch == '\r' {
				flush()
				continue
			}
			if ch == '>' || ch == '<' {
				// Keep fd prefix like "1>" or "2>>" attached to operator, otherwise flush.
				curStr := cur.String()
				if curStr != "1" && curStr != "2" && curStr != "&" {
					flush()
				}
				cur.WriteByte(ch)
				if ch == '>' && i+1 < n && seg[i+1] == '>' {
					i++
					cur.WriteByte('>')
				}
				flush()
				continue
			}
		}

		cur.WriteByte(ch)
	}

	flush()
	return tokens
}

// findCommandIndex returns the index of the primary executable token, skipping leading ENV=val assignments and sudo/env.
func findCommandIndex(tokens []string) int {
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok == "" {
			continue
		}
		if isRedirectionOp(tok) {
			i++ // skip redirection target
			continue
		}
		if strings.Contains(tok, "=") && !strings.HasPrefix(tok, "=") && !strings.Contains(tok, "/") {
			continue
		}
		base := path.Base(tok)
		if base == "sudo" || base == "env" || base == "command" {
			continue
		}
		return i
	}
	return -1
}

func isRedirectionOp(tok string) bool {
	return tok == ">" || tok == ">>" || tok == "1>" || tok == "1>>" || tok == "2>" || tok == "2>>" || tok == "&>" || tok == "&>>" || tok == "<"
}

func isOutputRedirectionOp(tok string) bool {
	return tok == ">" || tok == ">>" || tok == "1>" || tok == "1>>" || tok == "2>" || tok == "2>>" || tok == "&>" || tok == "&>>"
}

func hasOutputRedirection(tokens []string) bool {
	for _, tok := range tokens {
		if isOutputRedirectionOp(tok) {
			return true
		}
	}
	return false
}

func extractOutputRedirectionTargets(tokens []string) []string {
	var targets []string
	for i := 0; i < len(tokens); i++ {
		if isOutputRedirectionOp(tokens[i]) && i+1 < len(tokens) {
			targets = append(targets, tokens[i+1])
			i++
		}
	}
	return targets
}

func extractInputRedirectionTargets(tokens []string) []string {
	var targets []string
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "<" && i+1 < len(tokens) {
			targets = append(targets, tokens[i+1])
			i++
		}
	}
	return targets
}

// extractPathArgs extracts non-flag arguments while skipping redirection operators and their targets.
func extractPathArgs(args []string) []string {
	var res []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if isRedirectionOp(arg) {
			i++ // skip redirection target
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		res = append(res, arg)
	}
	return res
}

// extractScriptCandidates splits an inline script string by common delimiters to extract candidate file paths.
func extractScriptCandidates(script string) []string {
	f := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '\'', '"', '`', '(', ')', '[', ']', '{', '}', ',', ';', '=', '+':
			return true
		default:
			return false
		}
	}
	return strings.FieldsFunc(script, f)
}
