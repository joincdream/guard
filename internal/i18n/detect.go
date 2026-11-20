package i18n

import (
	"os"
	"strings"
)

const DefaultLang = "en"

// DetectLocale determines the preferred language code from environment variables.
// Priority: GUARD_LANG -> LC_ALL -> LC_MESSAGES -> LANG -> DefaultLang ("en").
func DetectLocale() string {
	return DetectLocaleFromLookup(os.Getenv)
}

// DetectLocaleFromLookup resolves the first supported language using a custom getenv function (for testability).
// Unsupported, C, POSIX, or empty values are skipped, falling back to DefaultLang ("en").
func DetectLocaleFromLookup(getenv func(string) string) string {
	for _, key := range []string{"GUARD_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if lang := ParseLocale(getenv(key)); lang != "" {
			if _, ok := catalogs[lang]; ok {
				return lang
			}
		}
	}
	return DefaultLang
}

// ParseLocale extracts the primary ISO 639-1 language code from a POSIX locale string
// (e.g., "ko_KR.UTF-8" -> "ko", "en-US" -> "en"). Returns "" for C/POSIX/empty values.
func ParseLocale(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Strip encoding (.UTF-8) and modifier (@euro)
	if idx := strings.IndexAny(s, ".@"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "c" || s == "posix" {
		return ""
	}
	// Extract language prefix before '_' or '-'
	if idx := strings.IndexAny(s, "_-"); idx >= 0 {
		s = s[:idx]
	}
	return s
}
