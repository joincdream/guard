# [01_core_policy_contract.md] Core 정책 평가 엔진 & Policy 로더 계약서

> **대상 독자**: QA (블랙박스 테스트 작성 기준)
> **대상 패키지**: `guard/internal/core`, `guard/internal/hub/policy`

---

## 1. `guard/internal/core`

### 1.1 공개 상수
```go
const (
    ModeDev  = "DEV"
    ModeTest = "TEST"
    ModePlan = "PLAN"
    ModeAll  = "ALL"
    ModeQA   = "QA"
)
```

### 1.2 공개 DTO / Struct
```go
type Rule struct {
    Name            string   `yaml:"name"`
    Mode            string   `yaml:"mode"` // "DEV", "PLAN", "TEST", 또는 ""(전체 모드 적용)
    Type            string   `yaml:"type"` // "read" 또는 "write"
    Patterns        []string `yaml:"patterns"`
    ExcludePatterns []string `yaml:"exclude_patterns"`
    Message         string   `yaml:"message"`
}

type Policy struct {
    Version string `yaml:"version"`
    Rules   []Rule `yaml:"rules"`
}

type EvalRequest struct {
    Mode        string
    ToolName    string
    TargetPath  string
    CommandLine string
}

type Decision struct {
    Allowed  bool
    RuleName string
    Reason   string
}
```

### 1.3 공개 함수 시그니처 및 동작 계약

| 함수 시그니처 | 동작 계약 |
| :--- | :--- |
| `NormalizeMode(mode string) string` | 앞뒤 공백 제거 후 대문자 변환. 빈 문자열은 `""` 반환 |
| `IsBypassMode(mode string) bool` | `NormalizeMode(mode)`가 `"ALL"` 또는 `"QA"`이면 `true` |
| `EvaluateRequest(policy *Policy, req EvalRequest) Decision` | 요청을 정책에 대조해 허용/차단 판정 (순수 함수, I/O 없음) |
| `Evaluate(mode string, policy *Policy, toolName, targetPath, cmdLine string) Decision` | `EvaluateRequest`의 호환 래퍼 (동작 100% 동일) |
| `MatchPattern(targetPath, pattern string) bool` | 경로가 패턴과 일치하는지 판별 |

### 1.4 핵심 평가 규칙 (`EvaluateRequest`)
1. **Bypass**: `IsBypassMode(req.Mode) == true` 또는 `policy == nil` 또는 `len(policy.Rules) == 0` → `Decision{Allowed: true}`.
2. **도구 기반 경로 검사**:
   - `ToolName == "read_file"` → `type == "read"` 규칙 평가
   - `ToolName == "write_file"` 또는 `"edit_file"` → `type == "write"` 규칙 평가
3. **셸 명령어 검사** (`strings.TrimSpace(req.CommandLine) != ""`):
   - 제어 연산자(`|`, `||`, `&&`, `;`, `\n`)로 세그먼트 분리 (따옴표·이스케이프 존중).
   - **안전 세그먼트 화이트리스트** (출력 리다이렉션이 없을 때만): `git status`, `git branch`, `git commit`, `git log` (`-p`/`--patch`/`-u` 제외), `go build`, `go vet`.
   - **읽기 차단 대상**:
     - 파일 뷰어(`cat`, `head`, `tail`, `more`, `less`, `strings`, `nl`, `tac`, `bat`) 인자
     - 입력 리다이렉션(`< target`)
     - Git 조회(`git show`, `git diff`, `git checkout`, `git log -p/--patch/-u`, `rev:path` 문법 포함)
     - 인라인 스크립트(`python`/`python3`/`node`/`ruby`/`perl`/`bash`/`sh`/`zsh` + `-c`/`-e`/`--eval`) 본문 내 경로 후보
   - **쓰기 차단 대상**:
     - 출력 리다이렉션(`>`, `>>`, `1>`, `2>`, `&>` 등)
     - `tee` 인자, `sed -i` / `sed --in-place` 인자
     - 인라인 스크립트(`-c`/`-e`/`--eval`) 본문 내 경로 후보
4. **패턴 매칭 (`MatchPattern`)**:
   - 지원 문법: 접미사(`*_test.go`), 표준 glob(`*.ts`, `config/*.json`), 다중 디렉토리 와일드카드(`**/tests/**`, `src/internal/**`).
   - `ExcludePatterns`에 하나라도 매칭되면 해당 규칙의 차단을 면제.
   - 규칙은 선언 순으로 평가하며, 최초 매칭된 규칙의 `Name`과 `Message`를 반환.

---

## 2. `guard/internal/hub/policy`

### 2.1 공개 상수 & 타입
```go
const (
    EnvPolicyPath  = "GUARD_POLICY_PATH"
    BuiltinSource  = "builtin"
    PolicyFileName = "guard-policy.yaml"
)

type Policy = core.Policy
type Rule = core.Rule

type LoadOptions struct {
    EnvPath       string
    CWD           string
    WorkspaceRoot string
    HomeDir       string
}

type LoadResult struct {
    Policy   *Policy
    Source   string
    Warnings []string
}
```

### 2.2 공개 함수 시그니처 및 동작 계약

| 함수 시그니처 | 동작 계약 |
| :--- | :--- |
| `DefaultPolicy() *Policy` | 내장 폴백 정책 반환 (`DEV` 테스트 읽기 차단, `PLAN` 소스 쓰기 차단, `TEST` 프로덕션 읽기 차단) |
| `LoadFile(path string) (*Policy, error)` | 단일 YAML 파일 파싱 |
| `LoadDetailedWithOptions(opts LoadOptions) LoadResult` | 우선순위 순으로 최초 유효 정책 로드. 파일은 존재하나 YAML 파싱 실패 또는 규칙 0건이면 `Warnings`에 기록 후 다음 후보로 진행. 모두 실패 시 `DefaultPolicy()`, `Source: "builtin"` 반환 |
| `LoadDetailed(workspaceRoot string) LoadResult` | 현재 환경(`GUARD_POLICY_PATH`, `os.Getwd()`, `os.UserHomeDir()`)으로 `LoadDetailedWithOptions` 호출 |
| `LoadWithOptions(opts LoadOptions) (*Policy, string)` | `LoadDetailedWithOptions` 호환 래퍼 |
| `Load(workspaceRoot string) (*Policy, string)` | `LoadDetailed` 호환 래퍼 |
| `ResolveWorkspaceMode(workspaceRoot string) string` | 폴백 모드 해석: `AGENT_MODE` → `AGENT_ROLE` → `<workspaceRoot>/.agents/current_mode` → `core.ModeDev` |

### 2.3 정책 파일 탐색 우선순위
1. `opts.EnvPath` (`$GUARD_POLICY_PATH`)
2. `<opts.CWD>/guard-policy.yaml`
3. `<opts.WorkspaceRoot>/.agents/guard-policy.yaml`
4. `<opts.HomeDir>/.config/guard/guard-policy.yaml`
5. Built-in fallback (`DefaultPolicy()`)
