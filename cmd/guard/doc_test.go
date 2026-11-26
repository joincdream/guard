package main

import (
	"bytes"
	"strings"
	"testing"

	"guard/internal/i18n"
)

func TestPolicyDocI18n(t *testing.T) {
	prev := i18n.Lang()
	defer i18n.SetLang(prev)

	i18n.SetLang("en")
	enDoc := PolicyDoc()
	for _, want := range []string{
		"# Guard Policy Specification & Few-Shot Sample",
		`type: "command"`,
		`"go test*"`,
		`"CommandLine":"go test ./..."`,
	} {
		if !strings.Contains(enDoc, want) {
			t.Errorf("en PolicyDoc() missing %q", want)
		}
	}

	i18n.SetLang("ko")
	var out bytes.Buffer
	app := &cliApp{stdout: &out}
	if code := app.run([]string{"doc"}); code != 0 {
		t.Fatalf("app.run([doc]) = %d, want 0", code)
	}
	koDoc := out.String()
	for _, want := range []string{
		"# Guard 정책 명세 및 Few-Shot 샘플",
		`type: "command"`,
		`"go test*"`,
		`"CommandLine":"go test ./..."`,
	} {
		if !strings.Contains(koDoc, want) {
			t.Errorf("ko guard doc output missing %q", want)
		}
	}
}
