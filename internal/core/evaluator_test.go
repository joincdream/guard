package core

import "testing"

func testPolicy() *Policy {
	return &Policy{
		Version: "1.0",
		Rules: []Rule{
			{
				Name: "guard-test-files-read",
				Mode: "DEV",
				Type: "read",
				Patterns: []string{
					"*_test.go",
					"*.test.ts",
					"*.test.js",
					"*.spec.ts",
					"*.spec.js",
					"**/testutil/**",
					"**/tests/**",
					"**/000/**",
				},
				Message: "[POLICY VIOLATION] DEV mode test file read blocked",
			},
			{
				Name: "guard-source-code-write",
				Mode: "PLAN",
				Type: "write",
				Patterns: []string{
					"src/internal/**",
					"src/cmd/**",
					"src/web/**",
					"src/docker/**",
				},
				Message: "[POLICY VIOLATION] PLAN mode source code write blocked",
			},
			{
				Name: "guard-production-code-read",
				Mode: "TEST",
				Type: "read",
				Patterns: []string{
					"src/internal/**",
					"src/cmd/**",
				},
				ExcludePatterns: []string{
					"*_test.go",
					"*.test.ts",
					"*.test.js",
					"*.spec.ts",
					"*.spec.js",
					"**/testutil/**",
					"**/contract/**",
				},
				Message: "[POLICY VIOLATION] TEST mode production code read blocked",
			},
		},
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		// Suffix patterns
		{name: "go test suffix", path: "pkg/service/user_test.go", pattern: "*_test.go", want: true},
		{name: "go prod file not matching test suffix", path: "pkg/service/user.go", pattern: "*_test.go", want: false},
		{name: "ts spec suffix", path: "web/app.spec.ts", pattern: "*.spec.ts", want: true},
		{name: "relative dot slash prefix", path: "./user_test.go", pattern: "*_test.go", want: true},

		// Multi-directory wildcards (**)
		{name: "leading double star root level", path: "tests/integration.go", pattern: "**/tests/**", want: true},
		{name: "leading double star nested", path: "src/pkg/tests/e2e/run.go", pattern: "**/tests/**", want: true},
		{name: "leading double star absolute", path: "/workspace/tests/case1.go", pattern: "**/tests/**", want: true},
		{name: "leading double star non-matching dir", path: "src/mytests/case1.go", pattern: "**/tests/**", want: false},
		{name: "testutil nested", path: "src/internal/testutil/mock.go", pattern: "**/testutil/**", want: true},
		{name: "000 directory", path: "docs/000/secret.md", pattern: "**/000/**", want: true},

		// Directory prefix patterns (src/internal/**)
		{name: "dir prefix relative", path: "src/internal/auth/handler.go", pattern: "src/internal/**", want: true},
		{name: "dir prefix absolute", path: "/home/user/repo/src/internal/auth/handler.go", pattern: "src/internal/**", want: true},
		{name: "dir prefix boundary no false positive", path: "src/internal_backup/auth.go", pattern: "src/internal/**", want: false},
		{name: "dir prefix unrelated", path: "doc/01_requirements.md", pattern: "src/internal/**", want: false},

		// Standard glob
		{name: "standard glob base", path: "src/app.ts", pattern: "*.ts", want: true},
		{name: "standard glob mismatch", path: "src/app.go", pattern: "*.ts", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchPattern(tc.path, tc.pattern)
			if got != tc.want {
				t.Errorf("MatchPattern(%q, %q) = %v, want %v", tc.path, tc.pattern, got, tc.want)
			}
		})
	}
}

func TestEvaluate_FileTools(t *testing.T) {
	pol := testPolicy()

	tests := []struct {
		name        string
		mode        string
		toolName    string
		targetPath  string
		wantAllowed bool
		wantRule    string
	}{
		// DEV mode
		{
			name:        "DEV read _test.go denied",
			mode:        "DEV",
			toolName:    "read_file",
			targetPath:  "src/internal/user_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV read tests dir denied",
			mode:        "DEV",
			toolName:    "read_file",
			targetPath:  "tests/e2e/login.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV read production file allowed",
			mode:        "DEV",
			toolName:    "read_file",
			targetPath:  "src/internal/user.go",
			wantAllowed: true,
		},
		{
			name:        "DEV write production file allowed",
			mode:        "DEV",
			toolName:    "write_file",
			targetPath:  "src/internal/user.go",
			wantAllowed: true,
		},
		{
			name:        "DEV lowercase mode normalized",
			mode:        " dev ",
			toolName:    "read_file",
			targetPath:  "pkg/foo_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},

		// PLAN mode
		{
			name:        "PLAN write src/internal denied",
			mode:        "PLAN",
			toolName:    "write_file",
			targetPath:  "src/internal/service.go",
			wantAllowed: false,
			wantRule:    "guard-source-code-write",
		},
		{
			name:        "PLAN edit src/cmd denied",
			mode:        "PLAN",
			toolName:    "edit_file",
			targetPath:  "/repo/src/cmd/main.go",
			wantAllowed: false,
			wantRule:    "guard-source-code-write",
		},
		{
			name:        "PLAN write doc allowed",
			mode:        "PLAN",
			toolName:    "write_file",
			targetPath:  "doc/02_architecture.md",
			wantAllowed: true,
		},
		{
			name:        "PLAN read src/internal allowed",
			mode:        "PLAN",
			toolName:    "read_file",
			targetPath:  "src/internal/service.go",
			wantAllowed: true,
		},

		// TEST mode (with exclude_patterns)
		{
			name:        "TEST read prod code denied",
			mode:        "TEST",
			toolName:    "read_file",
			targetPath:  "src/internal/service.go",
			wantAllowed: false,
			wantRule:    "guard-production-code-read",
		},
		{
			name:        "TEST read test file excluded and allowed",
			mode:        "TEST",
			toolName:    "read_file",
			targetPath:  "src/internal/service_test.go",
			wantAllowed: true,
		},
		{
			name:        "TEST read testutil excluded and allowed",
			mode:        "TEST",
			toolName:    "read_file",
			targetPath:  "src/internal/testutil/helper.go",
			wantAllowed: true,
		},
		{
			name:        "TEST read contract excluded and allowed",
			mode:        "TEST",
			toolName:    "read_file",
			targetPath:  "src/internal/contract/interface.go",
			wantAllowed: true,
		},

		// ALL / QA mode
		{
			name:        "ALL mode allows test read",
			mode:        "ALL",
			toolName:    "read_file",
			targetPath:  "src/internal/service_test.go",
			wantAllowed: true,
		},
		{
			name:        "QA mode allows source write",
			mode:        "QA",
			toolName:    "write_file",
			targetPath:  "src/internal/service.go",
			wantAllowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := Evaluate(tc.mode, pol, tc.toolName, tc.targetPath, "")
			if dec.Allowed != tc.wantAllowed {
				t.Fatalf("Evaluate() Allowed = %v, want %v (reason: %q)", dec.Allowed, tc.wantAllowed, dec.Reason)
			}
			if !tc.wantAllowed && dec.RuleName != tc.wantRule {
				t.Errorf("Evaluate() RuleName = %q, want %q", dec.RuleName, tc.wantRule)
			}
		})
	}
}

func TestEvaluate_CommandRules(t *testing.T) {
	pol := testPolicy()

	tests := []struct {
		name        string
		mode        string
		cmdLine     string
		wantAllowed bool
		wantRule    string
	}{
		// Safe command whitelist
		{
			name:        "safe git status",
			mode:        "DEV",
			cmdLine:     "git status",
			wantAllowed: true,
		},
		{
			name:        "safe git branch",
			mode:        "DEV",
			cmdLine:     "git branch -a",
			wantAllowed: true,
		},
		{
			name:        "safe go build",
			mode:        "DEV",
			cmdLine:     "go build ./...",
			wantAllowed: true,
		},
		{
			name:        "safe go vet",
			mode:        "DEV",
			cmdLine:     "go vet ./...",
			wantAllowed: true,
		},
		{
			name:        "safe git log summary",
			mode:        "DEV",
			cmdLine:     "git log --oneline -n 5",
			wantAllowed: true,
		},
		{
			name:        "safe go test execution allowed in DEV",
			mode:        "DEV",
			cmdLine:     "go test -v ./internal/core/...",
			wantAllowed: true,
		},

		// DEV mode file viewers
		{
			name:        "DEV cat test file denied",
			mode:        "DEV",
			cmdLine:     "cat pkg/auth_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV head test file denied",
			mode:        "DEV",
			cmdLine:     "head -n 50 tests/e2e.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV tail with wildcard denied",
			mode:        "DEV",
			cmdLine:     "tail -n 20 pkg/*_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV strings on testutil denied",
			mode:        "DEV",
			cmdLine:     "strings src/internal/testutil/fixture.bin",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV cat prod file allowed",
			mode:        "DEV",
			cmdLine:     "cat src/internal/auth.go",
			wantAllowed: true,
		},
		{
			name:        "DEV input redirection read denied",
			mode:        "DEV",
			cmdLine:     "wc -l < pkg/auth_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},

		// Compound operators bypass detection (;, &&, ||, |)
		{
			name:        "DEV whitelist prefix bypass via && denied",
			mode:        "DEV",
			cmdLine:     "git status && cat pkg/auth_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV bypass via semicolon denied",
			mode:        "DEV",
			cmdLine:     "go build ./...; head -n 10 tests/spec.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV bypass via || denied",
			mode:        "DEV",
			cmdLine:     "false || less pkg/foo_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV bypass via pipe denied",
			mode:        "DEV",
			cmdLine:     "cat pkg/foo_test.go | grep TestAuth",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},

		// DEV mode Git inspectors
		{
			name:        "DEV git show rev:path denied",
			mode:        "DEV",
			cmdLine:     "git show HEAD:pkg/auth_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV git diff test file denied",
			mode:        "DEV",
			cmdLine:     "git diff HEAD~1 -- pkg/auth_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV git log -p test file denied",
			mode:        "DEV",
			cmdLine:     "git log -p pkg/auth_test.go",
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV git diff prod file allowed",
			mode:        "DEV",
			cmdLine:     "git diff src/internal/auth.go",
			wantAllowed: true,
		},

		// DEV mode inline scripts
		{
			name:        "DEV python -c test file read denied",
			mode:        "DEV",
			cmdLine:     `python3 -c "print(open('pkg/auth_test.go').read())"`,
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV node -e test file read denied",
			mode:        "DEV",
			cmdLine:     `node -e "console.log(require('fs').readFileSync('web/app.spec.ts','utf8'))"`,
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV bash -c test file read denied",
			mode:        "DEV",
			cmdLine:     `bash -c "cat pkg/auth_test.go"`,
			wantAllowed: false,
			wantRule:    "guard-test-files-read",
		},
		{
			name:        "DEV python -c benign calculation allowed",
			mode:        "DEV",
			cmdLine:     `python3 -c "print(2 + 2)"`,
			wantAllowed: true,
		},

		// PLAN mode write bypasses
		{
			name:        "PLAN redirect > src/internal denied",
			mode:        "PLAN",
			cmdLine:     `echo "package main" > src/internal/app.go`,
			wantAllowed: false,
			wantRule:    "guard-source-code-write",
		},
		{
			name:        "PLAN append >> src/cmd denied",
			mode:        "PLAN",
			cmdLine:     `echo "// edit" >> src/cmd/main.go`,
			wantAllowed: false,
			wantRule:    "guard-source-code-write",
		},
		{
			name:        "PLAN tee src/web denied",
			mode:        "PLAN",
			cmdLine:     `echo "hack" | tee -a src/web/index.ts`,
			wantAllowed: false,
			wantRule:    "guard-source-code-write",
		},
		{
			name:        "PLAN sed -i src/internal denied",
			mode:        "PLAN",
			cmdLine:     `sed -i 's/foo/bar/g' src/internal/service.go`,
			wantAllowed: false,
			wantRule:    "guard-source-code-write",
		},
		{
			name:        "PLAN compound whitelist + redirect denied",
			mode:        "PLAN",
			cmdLine:     `git status && echo "x" > src/internal/app.go`,
			wantAllowed: false,
			wantRule:    "guard-source-code-write",
		},
		{
			name:        "PLAN redirect to docs allowed",
			mode:        "PLAN",
			cmdLine:     `echo "# Plan" > doc/plan.md`,
			wantAllowed: true,
		},

		// TEST mode shell read bypasses
		{
			name:        "TEST cat prod code denied",
			mode:        "TEST",
			cmdLine:     "cat src/internal/service.go",
			wantAllowed: false,
			wantRule:    "guard-production-code-read",
		},
		{
			name:        "TEST cat test file allowed via exclude_patterns",
			mode:        "TEST",
			cmdLine:     "cat src/internal/service_test.go",
			wantAllowed: true,
		},
		{
			name:        "TEST cat contract file allowed via exclude_patterns",
			mode:        "TEST",
			cmdLine:     "cat src/internal/contract/api.go",
			wantAllowed: true,
		},

		// ALL mode bypasses everything
		{
			name:        "ALL mode allows cat test file",
			mode:        "ALL",
			cmdLine:     "cat pkg/auth_test.go",
			wantAllowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := Evaluate(tc.mode, pol, "run_command", "", tc.cmdLine)
			if dec.Allowed != tc.wantAllowed {
				t.Fatalf("Evaluate(%q, cmd=%q) Allowed = %v, want %v (reason: %q)", tc.mode, tc.cmdLine, dec.Allowed, tc.wantAllowed, dec.Reason)
			}
			if !tc.wantAllowed && dec.RuleName != tc.wantRule {
				t.Errorf("Evaluate(%q, cmd=%q) RuleName = %q, want %q", tc.mode, tc.cmdLine, dec.RuleName, tc.wantRule)
			}
		})
	}
}

func TestEvaluate_NilPolicy(t *testing.T) {
	dec := Evaluate("DEV", nil, "read_file", "pkg/foo_test.go", "cat pkg/foo_test.go")
	if !dec.Allowed {
		t.Fatalf("expected nil policy to allow, got deny: %q", dec.Reason)
	}
}

func TestMatchCommandPattern(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		pattern string
		want    bool
	}{
		{name: "exact plain match", cmd: "go test", pattern: "go test", want: true},
		{name: "plain prefix with args", cmd: "go test -v ./internal/core/...", pattern: "go test", want: true},
		{name: "plain prefix boundary no partial word match", cmd: "go testdata", pattern: "go test", want: false},
		{name: "wildcard star matches slashes and spaces", cmd: "go test -v ./internal/core/...", pattern: "go test*", want: true},
		{name: "wildcard star matches bare command", cmd: "go test", pattern: "go test*", want: true},
		{name: "make test wildcard", cmd: "make test-unit", pattern: "make test*", want: true},
		{name: "make build allowed against make test*", cmd: "make build", pattern: "make test*", want: false},
		{name: "script path normalization", cmd: "./scripts/run_tests.sh --ci", pattern: "scripts/run_tests.sh*", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchCommandPattern(tc.cmd, tc.pattern)
			if got != tc.want {
				t.Errorf("MatchCommandPattern(%q, %q) = %v, want %v", tc.cmd, tc.pattern, got, tc.want)
			}
		})
	}
}

func TestEvaluate_CommandExecRules(t *testing.T) {
	pol := &Policy{
		Version: "1.0",
		Rules: []Rule{
			{
				Name: "guard-test-command-exec",
				Mode: "DEV",
				Type: "command",
				Patterns: []string{
					"go test*",
					"make test*",
					"pytest",
				},
				ExcludePatterns: []string{
					"go test -list*",
				},
				Message: "[POLICY VIOLATION] DEV mode test execution blocked",
			},
			{
				Name: "guard-plan-git-commit",
				Mode: "PLAN",
				Type: "exec",
				Patterns: []string{
					"git commit*",
				},
				Message: "[POLICY VIOLATION] PLAN mode git commit blocked",
			},
		},
	}

	tests := []struct {
		name        string
		mode        string
		cmdLine     string
		wantAllowed bool
		wantRule    string
	}{
		{
			name:        "DEV bare go test denied",
			mode:        "DEV",
			cmdLine:     "go test",
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV go test with flags and package denied",
			mode:        "DEV",
			cmdLine:     "go test -v ./internal/core/...",
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV go test with env vars and absolute binary path denied",
			mode:        "DEV",
			cmdLine:     "CGO_ENABLED=0 /usr/local/go/bin/go test ./...",
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV go -C dir test denied",
			mode:        "DEV",
			cmdLine:     "go -C internal/core test -v .",
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV chained go build && go test denied",
			mode:        "DEV",
			cmdLine:     "go build ./... && go test ./...",
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV subshell parens (cd pkg && go test) denied",
			mode:        "DEV",
			cmdLine:     "(cd internal/core && go test ./...)",
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV bash -c go test bypass denied",
			mode:        "DEV",
			cmdLine:     `bash -lc "go test -v ./..."`,
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV command substitution $(go test) denied",
			mode:        "DEV",
			cmdLine:     `out=$(go test ./...)`,
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV timeout wrapper around go test denied",
			mode:        "DEV",
			cmdLine:     "timeout 30s go test ./...",
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV python subprocess go test bypass denied",
			mode:        "DEV",
			cmdLine:     `python3 -c "import subprocess; subprocess.run(['go', 'test', './...'])"`,
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV make test with -C flag denied",
			mode:        "DEV",
			cmdLine:     "make -C . test",
			wantAllowed: false,
			wantRule:    "guard-test-command-exec",
		},
		{
			name:        "DEV excluded go test -list allowed",
			mode:        "DEV",
			cmdLine:     "go test -list . ./internal/core",
			wantAllowed: true,
		},
		{
			name:        "DEV go build and go vet remain allowed",
			mode:        "DEV",
			cmdLine:     "go build ./... && go vet ./...",
			wantAllowed: true,
		},
		{
			name:        "DEV make build remains allowed",
			mode:        "DEV",
			cmdLine:     "make build",
			wantAllowed: true,
		},
		{
			name:        "DEV command -v go allowed",
			mode:        "DEV",
			cmdLine:     "command -v go",
			wantAllowed: true,
		},
		{
			name:        "DEV string mention in git commit or handoff message not falsely blocked",
			mode:        "DEV",
			cmdLine:     `guard handoff send --to test -m "Please run go test ./..." && git commit -m "block go test in dev"`,
			wantAllowed: true,
		},
		{
			name:        "TEST mode allows go test",
			mode:        "TEST",
			cmdLine:     "go test -v ./...",
			wantAllowed: true,
		},
		{
			name:        "ALL mode bypasses command rules",
			mode:        "ALL",
			cmdLine:     "go test -v ./...",
			wantAllowed: true,
		},
		{
			name:        "PLAN mode blocks git commit even though git commit is in safe read/write list",
			mode:        "PLAN",
			cmdLine:     `git commit -m "wip"`,
			wantAllowed: false,
			wantRule:    "guard-plan-git-commit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := Evaluate(tc.mode, pol, "run_command", "", tc.cmdLine)
			if dec.Allowed != tc.wantAllowed {
				t.Fatalf("Evaluate(%q, cmd=%q) Allowed = %v, want %v (reason: %q)", tc.mode, tc.cmdLine, dec.Allowed, tc.wantAllowed, dec.Reason)
			}
			if !tc.wantAllowed && dec.RuleName != tc.wantRule {
				t.Errorf("Evaluate(%q, cmd=%q) RuleName = %q, want %q", tc.mode, tc.cmdLine, dec.RuleName, tc.wantRule)
			}
		})
	}
}
