package core

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	compiledGlobCache    sync.Map // map[string]*regexp.Regexp
	compiledCmdGlobCache sync.Map // map[string]*regexp.Regexp
)

func getCompiledGlob(pattern string) (*regexp.Regexp, error) {
	if cached, ok := compiledGlobCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := globToRegexp(pattern)
	if err != nil {
		return nil, err
	}
	compiledGlobCache.Store(pattern, re)
	return re, nil
}

// MatchPattern checks whether targetPath matches the given glob/wildcard pattern.
// Supports suffix patterns (*_test.go), standard globs (*.ts), and multi-directory wildcards (**/tests/**, src/internal/**).
func MatchPattern(targetPath string, pattern string) bool {
	cleanPath := normalizePath(targetPath)
	cleanPattern := normalizePath(pattern)

	if cleanPath == "" || cleanPattern == "" {
		return false
	}

	// 1. Direct suffix match (e.g. *_test.go, *.spec.ts)
	if strings.HasPrefix(cleanPattern, "*") && !strings.Contains(cleanPattern[1:], "*") && !strings.Contains(cleanPattern, "/") {
		if strings.HasSuffix(cleanPath, cleanPattern[1:]) {
			return true
		}
	}

	// 2. Standard glob match against full path and base filename
	if matched, _ := path.Match(cleanPattern, cleanPath); matched {
		return true
	}
	if !strings.Contains(cleanPattern, "/") {
		if matched, _ := path.Match(cleanPattern, path.Base(cleanPath)); matched {
			return true
		}
	}

	// 3. Multi-directory wildcard (**) match
	if strings.Contains(cleanPattern, "**") {
		if re, err := getCompiledGlob(cleanPattern); err == nil && re.MatchString(cleanPath) {
			return true
		}
	}

	// 4. Relative directory prefix match for patterns ending with /** (e.g. src/internal/**)
	if strings.HasSuffix(cleanPattern, "/**") {
		prefix := strings.TrimSuffix(cleanPattern, "/**")
		if prefix != "" && !strings.Contains(prefix, "*") {
			if cleanPath == prefix || strings.HasPrefix(cleanPath, prefix+"/") || strings.Contains(cleanPath, "/"+prefix+"/") || strings.HasSuffix(cleanPath, "/"+prefix) {
				return true
			}
		}
	}

	return false
}

// evaluatePathRules evaluates a path against all policy rules matching currentMode and actionType.
// Returns (ruleName, violationMessage) on deny, or ("", "") on allow.
func evaluatePathRules(policy *Policy, currentMode string, actionType string, targetPath string) (string, string) {
	if policy == nil || strings.TrimSpace(targetPath) == "" {
		return "", ""
	}

	for _, rule := range policy.Rules {
		if rule.Mode != "" && !strings.EqualFold(rule.Mode, currentMode) {
			continue
		}
		if rule.Type != "" && !strings.EqualFold(rule.Type, actionType) {
			continue
		}

		for _, pattern := range rule.Patterns {
			if MatchPattern(targetPath, pattern) {
				if isExcluded(targetPath, rule.ExcludePatterns) {
					break
				}
				return rule.Name, rule.Message
			}
		}
	}

	return "", ""
}

func isExcluded(targetPath string, excludePatterns []string) bool {
	for _, ex := range excludePatterns {
		if MatchPattern(targetPath, ex) {
			return true
		}
	}
	return false
}

func normalizePath(p string) string {
	s := strings.TrimSpace(p)
	s = strings.Trim(s, `"'`)
	if s == "" {
		return ""
	}
	s = filepath.ToSlash(s)
	// Strip leading "./" segments for consistent relative matching.
	for strings.HasPrefix(s, "./") {
		s = strings.TrimPrefix(s, "./")
	}
	// Collapse redundant slashes while preserving leading slash if absolute.
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	return s
}

// globToRegexp converts a glob pattern containing ** into an anchored Go regular expression.
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")

	i := 0
	n := len(pattern)
	for i < n {
		// Handle leading "**/" -> matches zero or more directories (e.g. "tests/a.go" or "a/b/tests/a.go")
		if i == 0 && strings.HasPrefix(pattern, "**/") {
			b.WriteString("(?:.*/)?")
			i += 3
			continue
		}
		// Handle middle "/**/" -> matches "/" or "/.../"
		if strings.HasPrefix(pattern[i:], "/**/") {
			b.WriteString("(?:/|/.*/)")
			i += 4
			continue
		}
		// Handle trailing "/**" -> matches "/..." or end of string
		if pattern[i:] == "/**" {
			b.WriteString("(?:/.*)?")
			i += 3
			continue
		}
		// Handle standalone "**"
		if strings.HasPrefix(pattern[i:], "**") {
			b.WriteString(".*")
			i += 2
			continue
		}
		// Handle single "*"
		if pattern[i] == '*' {
			b.WriteString("[^/]*")
			i++
			continue
		}
		// Handle "?"
		if pattern[i] == '?' {
			b.WriteString("[^/]")
			i++
			continue
		}

		b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		i++
	}

	b.WriteString("$")
	return regexp.Compile(b.String())
}

func getCompiledCommandGlob(pattern string) (*regexp.Regexp, error) {
	if cached, ok := compiledCmdGlobCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := commandGlobToRegexp(pattern)
	if err != nil {
		return nil, err
	}
	compiledCmdGlobCache.Store(pattern, re)
	return re, nil
}

func commandGlobToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func normalizeCommandSpaces(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// MatchCommandPattern checks whether a normalized command string (e.g. "go test -v ./...")
// matches a command rule pattern.
//   - Plain patterns without '*' or '?' (e.g. "go test", "pytest") match either the exact command
//     or a token-delimited prefix ("go test ...").
//   - Wildcard patterns containing '*' or '?' (e.g. "go test*", "make test*") match across the
//     entire command string, including spaces and path separators.
func MatchCommandPattern(cmdStr string, pattern string) bool {
	cleanCmd := normalizeCommandSpaces(cmdStr)
	cleanPattern := normalizeCommandSpaces(strings.Trim(strings.TrimSpace(pattern), `"'`))
	if cleanCmd == "" || cleanPattern == "" {
		return false
	}

	if strings.Contains(cleanPattern, "/") {
		for strings.HasPrefix(cleanCmd, "./") {
			cleanCmd = strings.TrimPrefix(cleanCmd, "./")
		}
		for strings.HasPrefix(cleanPattern, "./") {
			cleanPattern = strings.TrimPrefix(cleanPattern, "./")
		}
	}

	if !strings.ContainsAny(cleanPattern, "*?") {
		return cleanCmd == cleanPattern || strings.HasPrefix(cleanCmd, cleanPattern+" ")
	}

	re, err := getCompiledCommandGlob(cleanPattern)
	if err != nil {
		return false
	}
	return re.MatchString(cleanCmd)
}

func isCommandRuleType(ruleType string) bool {
	return strings.EqualFold(ruleType, RuleTypeCommand) || strings.EqualFold(ruleType, RuleTypeExec)
}

// evaluateExecCommandRules evaluates candidate normalized command strings against all
// command/exec policy rules matching currentMode.
func evaluateExecCommandRules(policy *Policy, currentMode string, candidates []string) (string, string) {
	if policy == nil || len(candidates) == 0 {
		return "", ""
	}

	for _, rule := range policy.Rules {
		if rule.Mode != "" && !strings.EqualFold(rule.Mode, currentMode) {
			continue
		}
		if !isCommandRuleType(rule.Type) {
			continue
		}

		matched := false
		for _, pattern := range rule.Patterns {
			for _, cand := range candidates {
				if MatchCommandPattern(cand, pattern) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			continue
		}
		if isCommandExcluded(candidates, rule.ExcludePatterns) {
			continue
		}
		return rule.Name, rule.Message
	}

	return "", ""
}

func isCommandExcluded(candidates []string, excludePatterns []string) bool {
	for _, ex := range excludePatterns {
		for _, cand := range candidates {
			if MatchCommandPattern(cand, ex) {
				return true
			}
		}
	}
	return false
}
