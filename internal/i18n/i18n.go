package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed locales/*.json
var localesFS embed.FS

var (
	mu         sync.RWMutex
	catalogs   map[string]map[string]string
	activeLang = DefaultLang
)

func init() {
	catalogs = loadEmbeddedCatalogs()
	activeLang = resolveSupported(DetectLocale())
}

func loadEmbeddedCatalogs() map[string]map[string]string {
	loaded := make(map[string]map[string]string)
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		return loaded
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := localesFS.ReadFile("locales/" + entry.Name())
		if err != nil {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		lang := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		loaded[strings.ToLower(lang)] = m
	}
	return loaded
}

func resolveSupported(lang string) string {
	code := ParseLocale(lang)
	if code != "" {
		if _, ok := catalogs[code]; ok {
			return code
		}
	}
	return DefaultLang
}

// Init re-detects the active language from environment variables.
func Init() {
	SetLang(DetectLocale())
}

// SetLang switches the active language. Unsupported codes fall back to DefaultLang ("en").
func SetLang(lang string) {
	mu.Lock()
	defer mu.Unlock()
	activeLang = resolveSupported(lang)
}

// Lang returns the currently active language code (e.g., "en", "ko").
func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return activeLang
}

// SupportedLangs returns a sorted list of embedded language codes.
func SupportedLangs() []string {
	mu.RLock()
	defer mu.RUnlock()
	langs := make([]string, 0, len(catalogs))
	for k := range catalogs {
		langs = append(langs, k)
	}
	sort.Strings(langs)
	return langs
}

// T looks up a translation key in the active catalog, falling back to DefaultLang ("en")
// and finally to the raw key itself. Optional args are formatted with fmt.Sprintf.
func T(key string, args ...any) string {
	mu.RLock()
	lang := activeLang
	tmpl, ok := catalogs[lang][key]
	if !ok && lang != DefaultLang {
		tmpl, ok = catalogs[DefaultLang][key]
	}
	mu.RUnlock()

	if !ok {
		tmpl = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(tmpl, args...)
	}
	return tmpl
}

// RelativeTime formats a timestamp as a localized relative duration string.
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return T("time.just_now")
	case d < time.Hour:
		return T("time.minutes_ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return T("time.hours_ago", int(d.Hours()))
	default:
		return t.Format("01-02")
	}
}
