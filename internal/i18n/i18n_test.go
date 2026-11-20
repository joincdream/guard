package i18n

import (
	"regexp"
	"testing"
	"time"
)

var fmtVerbRe = regexp.MustCompile(`%[%#+ 0-9.*-]*[vTtbcdoqxXUeEfFgGsp]`)

func extractFormatVerbs(s string) []string {
	raw := fmtVerbRe.FindAllString(s, -1)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if v == "%%" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func TestCatalogKeyAndFormatParity(t *testing.T) {
	en, ok := catalogs["en"]
	if !ok {
		t.Fatal("missing embedded en catalog")
	}
	ko, ok := catalogs["ko"]
	if !ok {
		t.Fatal("missing embedded ko catalog")
	}

	for k, enVal := range en {
		koVal, exists := ko[k]
		if !exists {
			t.Errorf("key %q exists in en.json but missing in ko.json", k)
			continue
		}
		enVerbs := extractFormatVerbs(enVal)
		koVerbs := extractFormatVerbs(koVal)
		if len(enVerbs) != len(koVerbs) {
			t.Errorf("format verb count mismatch for key %q: en=%v, ko=%v", k, enVerbs, koVerbs)
		}
	}

	for k := range ko {
		if _, exists := en[k]; !exists {
			t.Errorf("key %q exists in ko.json but missing in en.json", k)
		}
	}
}

func TestParseAndDetectLocale(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"ko_KR.UTF-8", "ko"},
		{"en_US.UTF-8", "en"},
		{"ko-KR", "ko"},
		{"ja_JP.UTF-8@euro", "ja"},
		{"C", ""},
		{"POSIX", ""},
		{"C.UTF-8", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ParseLocale(tc.raw); got != tc.want {
			t.Errorf("ParseLocale(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}

	env := map[string]string{
		"GUARD_LANG": "",
		"LC_ALL":     "",
		"LANG":       "ko_KR.UTF-8",
	}
	if got := DetectLocaleFromLookup(func(k string) string { return env[k] }); got != "ko" {
		t.Errorf("DetectLocaleFromLookup(LANG=ko_KR.UTF-8) = %q, want ko", got)
	}

	env["GUARD_LANG"] = "en"
	if got := DetectLocaleFromLookup(func(k string) string { return env[k] }); got != "en" {
		t.Errorf("DetectLocaleFromLookup(GUARD_LANG=en) = %q, want en", got)
	}
}

func TestTranslateAndFallback(t *testing.T) {
	prev := Lang()
	defer SetLang(prev)

	SetLang("ko")
	if got := T("time.just_now"); got != "방금 전" {
		t.Errorf("ko time.just_now = %q", got)
	}
	SetLang("en")
	if got := T("time.just_now"); got != "just now" {
		t.Errorf("en time.just_now = %q", got)
	}
	SetLang("fr") // unsupported -> fallback to en
	if Lang() != "en" {
		t.Errorf("unsupported lang should fallback to en, got %q", Lang())
	}
	if got := T("nonexistent.key"); got != "nonexistent.key" {
		t.Errorf("missing key should return raw key, got %q", got)
	}
}

func TestRelativeTime(t *testing.T) {
	prev := Lang()
	defer SetLang(prev)

	SetLang("en")
	if got := RelativeTime(time.Time{}); got != "-" {
		t.Errorf("zero time = %q, want -", got)
	}
	if got := RelativeTime(time.Now().Add(-10 * time.Second)); got != "just now" {
		t.Errorf("10s ago = %q, want just now", got)
	}
	if got := RelativeTime(time.Now().Add(-5 * time.Minute)); got != "5m ago" {
		t.Errorf("5m ago = %q, want 5m ago", got)
	}
	SetLang("ko")
	if got := RelativeTime(time.Now().Add(-2 * time.Hour)); got != "2시간 전" {
		t.Errorf("2h ago = %q, want 2시간 전", got)
	}
}
