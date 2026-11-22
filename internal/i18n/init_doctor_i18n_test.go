package i18n_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var fmtVerbRe = regexp.MustCompile(`%(?:\[\d+\])?[#+\- 0]*\d*(?:\.\d+)?[vTtbcdoOqxXUeEfFgGsp%]`)

func extractFormatVerbs(s string) []string {
	raw := fmtVerbRe.FindAllString(s, -1)
	var out []string
	for _, v := range raw {
		if v == "%%" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func loadLocaleJSON(t *testing.T, name string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("locales", name))
	if err != nil {
		t.Fatalf("ReadFile(locales/%s): %v", name, err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal(locales/%s): %v", name, err)
	}
	return m
}

// TC-I18N-001 & TC-I18N-002: ko.json / en.json 키 대칭성, cli.init.* / cli.doctor.* 존재, 포맷 동사 일치
func TestInitDoctor_I18N_CatalogSymmetryAndFormatVerbs(t *testing.T) {
	en := loadLocaleJSON(t, "en.json")
	ko := loadLocaleJSON(t, "ko.json")

	// TC-I18N-001: 전체 키 집합 대칭
	var missingInKo, missingInEn []string
	for k := range en {
		if _, ok := ko[k]; !ok {
			missingInKo = append(missingInKo, k)
		}
	}
	for k := range ko {
		if _, ok := en[k]; !ok {
			missingInEn = append(missingInEn, k)
		}
	}
	sort.Strings(missingInKo)
	sort.Strings(missingInEn)
	if len(missingInKo) > 0 {
		t.Errorf("keys present in en.json but missing in ko.json: %v", missingInKo)
	}
	if len(missingInEn) > 0 {
		t.Errorf("keys present in ko.json but missing in en.json: %v", missingInEn)
	}

	// cli.init.*, cli.doctor.*, cli.help 키 존재 및 비공백 확인
	initCount := 0
	doctorCount := 0
	for k, ev := range en {
		kv := ko[k]
		if strings.HasPrefix(k, "cli.init.") {
			initCount++
		}
		if strings.HasPrefix(k, "cli.doctor.") {
			doctorCount++
		}
		if strings.TrimSpace(ev) == "" {
			t.Errorf("en.json key %q has empty value", k)
		}
		if strings.TrimSpace(kv) == "" {
			t.Errorf("ko.json key %q has empty value", k)
		}

		// TC-I18N-002: 포맷 동사 개수 및 타입 일치
		enVerbs := extractFormatVerbs(ev)
		koVerbs := extractFormatVerbs(kv)
		if !reflect.DeepEqual(enVerbs, koVerbs) {
			t.Errorf("format verbs mismatch for key %q: en=%v vs ko=%v", k, enVerbs, koVerbs)
		}
	}

	if initCount == 0 {
		t.Errorf("expected cli.init.* keys in locale catalogs, found 0")
	}
	if doctorCount == 0 {
		t.Errorf("expected cli.doctor.* keys in locale catalogs, found 0")
	}
	if _, ok := en["cli.help"]; !ok {
		t.Errorf("missing cli.help in en.json")
	}
}
