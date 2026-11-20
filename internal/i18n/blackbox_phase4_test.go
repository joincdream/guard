package i18n_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"guard/internal/i18n"
)

// TC-P4-001: 로케일 감지 우선순위 (GUARD_LANG > LC_ALL > LC_MESSAGES > LANG > "en") 및 ParseLocale 경계
func TestPhase4_I18N_LocaleDetectionPriorityAndParsing(t *testing.T) {
	t.Run("priority_chain", func(t *testing.T) {
		steps := []struct {
			name string
			env  map[string]string
			want string
		}{
			{
				name: "GUARD_LANG_beats_all",
				env: map[string]string{
					"GUARD_LANG":  "ko",
					"LC_ALL":      "en_US.UTF-8",
					"LC_MESSAGES": "en_US.UTF-8",
					"LANG":        "en_US.UTF-8",
				},
				want: "ko",
			},
			{
				name: "LC_ALL_beats_LC_MESSAGES_and_LANG",
				env: map[string]string{
					"LC_ALL":      "ko_KR.UTF-8",
					"LC_MESSAGES": "en_US.UTF-8",
					"LANG":        "en_US.UTF-8",
				},
				want: "ko",
			},
			{
				name: "LC_MESSAGES_beats_LANG",
				env: map[string]string{
					"LC_MESSAGES": "ko_KR.UTF-8",
					"LANG":        "en_US.UTF-8",
				},
				want: "ko",
			},
			{
				name: "LANG_used_when_higher_empty",
				env: map[string]string{
					"LANG": "ko_KR.UTF-8",
				},
				want: "ko",
			},
			{
				name: "unsupported_higher_falls_back_to_supported_lower",
				env: map[string]string{
					"GUARD_LANG": "fr_FR.UTF-8",
					"LANG":       "ko_KR.UTF-8",
				},
				want: "ko",
			},
			{
				name: "unsupported_and_POSIX_fallback_to_en",
				env: map[string]string{
					"GUARD_LANG":  "fr_FR.UTF-8",
					"LC_ALL":      "POSIX",
					"LC_MESSAGES": "C.UTF-8",
					"LANG":        "ja_JP.UTF-8",
				},
				want: i18n.DefaultLang,
			},
			{
				name: "all_empty_defaults_to_en",
				env:  map[string]string{},
				want: i18n.DefaultLang,
			},
		}

		for _, tc := range steps {
			t.Run(tc.name, func(t *testing.T) {
				got := i18n.DetectLocaleFromLookup(func(k string) string { return tc.env[k] })
				if got != tc.want {
					t.Errorf("DetectLocaleFromLookup() = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("ParseLocale_boundaries", func(t *testing.T) {
		cases := []struct {
			raw  string
			want string
		}{
			{"ko_KR.UTF-8", "ko"},
			{"KO-kr", "ko"},
			{"en_US.UTF-8", "en"},
			{"fr_FR.UTF-8@euro", "fr"},
			{"C", ""},
			{"POSIX", ""},
			{"C.UTF-8", ""},
			{"   ", ""},
			{"", ""},
		}
		for _, tc := range cases {
			if got := i18n.ParseLocale(tc.raw); got != tc.want {
				t.Errorf("ParseLocale(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		}
	})

	t.Run("SupportedLangs_contains_en_and_ko", func(t *testing.T) {
		langs := i18n.SupportedLangs()
		if !slices.Contains(langs, "en") || !slices.Contains(langs, "ko") {
			t.Errorf("SupportedLangs() = %v, expected both 'en' and 'ko'", langs)
		}
	})
}

// TC-P4-002 & TC-P4-003: 번역 키 폴백 체인 및 Init() 환경변수 연동
func TestPhase4_I18N_TranslateFallbackAndInit(t *testing.T) {
	prev := i18n.Lang()
	defer i18n.SetLang(prev)

	t.Run("supported_langs_translate_known_key", func(t *testing.T) {
		i18n.SetLang("ko")
		if i18n.Lang() != "ko" {
			t.Fatalf("Lang() = %q, want ko", i18n.Lang())
		}
		if got := i18n.T("time.just_now"); got == "" || got == "time.just_now" {
			t.Errorf("expected Korean translation for time.just_now, got %q", got)
		}

		i18n.SetLang("en")
		if i18n.Lang() != "en" {
			t.Fatalf("Lang() = %q, want en", i18n.Lang())
		}
		if got := i18n.T("time.just_now"); got == "" || got == "time.just_now" {
			t.Errorf("expected English translation for time.just_now, got %q", got)
		}
	})

	t.Run("unsupported_lang_falls_back_to_default_en", func(t *testing.T) {
		for _, raw := range []string{"fr", "de", "", "   "} {
			i18n.SetLang(raw)
			if i18n.Lang() != i18n.DefaultLang {
				t.Errorf("SetLang(%q) -> Lang() = %q, want %q", raw, i18n.Lang(), i18n.DefaultLang)
			}
		}
	})

	t.Run("missing_key_returns_raw_key", func(t *testing.T) {
		i18n.SetLang("ko")
		const missingKey = "qa.nonexistent.translation.key"
		if got := i18n.T(missingKey); got != missingKey {
			t.Errorf("T(%q) = %q, want raw key %q", missingKey, got, missingKey)
		}
	})

	t.Run("Init_respects_GUARD_LANG", func(t *testing.T) {
		t.Setenv("GUARD_LANG", "ko")
		t.Setenv("LC_ALL", "en_US.UTF-8")
		i18n.Init()
		if i18n.Lang() != "ko" {
			t.Errorf("Init() with GUARD_LANG=ko set Lang() = %q, want ko", i18n.Lang())
		}

		t.Setenv("GUARD_LANG", "en")
		i18n.Init()
		if i18n.Lang() != "en" {
			t.Errorf("Init() with GUARD_LANG=en set Lang() = %q, want en", i18n.Lang())
		}
	})
}

// TC-P4-004: RelativeTime 시간 구간 경계 (zero, 미래, 초/분/시간/일 전)
func TestPhase4_I18N_RelativeTimeBoundaries(t *testing.T) {
	prev := i18n.Lang()
	defer i18n.SetLang(prev)

	now := time.Now()
	samples := []struct {
		name string
		ts   time.Time
	}{
		{name: "zero_time", ts: time.Time{}},
		{name: "future_10s", ts: now.Add(10 * time.Second)},
		{name: "now_0s", ts: now},
		{name: "30s_ago", ts: now.Add(-30 * time.Second)},
		{name: "5m_ago", ts: now.Add(-5 * time.Minute)},
		{name: "3h_ago", ts: now.Add(-3 * time.Hour)},
		{name: "2d_ago", ts: now.Add(-48 * time.Hour)},
	}

	for _, lang := range []string{"en", "ko"} {
		t.Run("lang_"+lang, func(t *testing.T) {
			i18n.SetLang(lang)
			for _, tc := range samples {
				t.Run(tc.name, func(t *testing.T) {
					got := i18n.RelativeTime(tc.ts)
					if strings.TrimSpace(got) == "" {
						t.Errorf("RelativeTime(%s) returned empty string in %s", tc.name, lang)
					}
					if tc.ts.IsZero() && got != "-" {
						t.Errorf("RelativeTime(zero) = %q, want '-'", got)
					}
				})
			}
		})
	}
}
