# [init-automation-qa-test-brief.md] 초기 설정 자동화 및 자가 진단 QA 테스트 가이드 (`guard init` / `guard doctor`)

> **대상 독자**: QA (`TEST` 모드 에이전트 및 검증 담당자)  
> **검증 대상**: `guard init`, `guard doctor`, `internal/hub/setup`, `launcher` bootstrap 연동, `findWorkspaceRoot()` symlink 정규화  
> **참고 문서**:
> - 계약 명세 (필수): [06_init_automation_contract.md](../spec/06_init_automation_contract.md), [03_adapters_cli_contract.md](../spec/03_adapters_cli_contract.md)
> - 설계서: [09_init_automation_design.md](../09_init_automation_design.md)
> - 개발 계획서: [init_automation_dev_plan.md](../dev/init_automation_dev_plan.md)

---

## 1. 구현된 기능 목록 (What's Built)

1. **원커맨드 워크스페이스 초기화 (`guard init`)**
   - 프로젝트 루트의 마커 파일(`go.mod` → `package.json` → `pyproject.toml` → `requirements.txt`)을 우선순위대로 감지해 `go` / `node` / `python` / `generic` 프리셋을 자동 선택합니다.
   - 4개 설정 파일을 한 번에 생성·업데이트합니다:
     1. `.agents/hooks.json`: 기존 다른 도구 키(`obsidian-chatlog` 등)를 유지한 채 `guard.PreToolUse` hook을 Safe Merge (Atomic Write: `.tmp` → `os.Rename`).
     2. `guard-policy.yaml`: 감지된 스택 프리셋에 맞는 정책 파일 생성.
     3. `.agents/rules/guard-workflow.md`: 상단에 `trigger: always_on` YAML frontmatter와 `<!-- guard-managed: v1 -->` 마커, `guard handoff` CLI 가이드가 포함된 룰 파일 생성.
     4. `.agents/current_mode`: 초기 모드(`DEV` 기본값, `DEV`/`PLAN`/`TEST`/`ALL` 지원) 저장.
   - 안전 옵션:
     - `--dry-run`: 파일 시스템을 전혀 건드리지 않고 실행 계획만 출력.
     - `--force`: 기존 `guard-policy.yaml` / `guard-workflow.md`를 `.bak`으로 백업한 뒤 재생성.
     - `--no-hooks`, `--no-policy`, `--no-rules`: 특정 항목만 선택적으로 스킵.
     - 여러 번 실행해도 결과가 같은 Idempotent 동작(이미 설정된 항목은 `skipped`).
2. **환경·설정 자가 진단 및 자동 복구 (`guard doctor`)**
   - 6개 체크 코드(`DOC-001` ~ `DOC-006`)로 워크스페이스 상태를 점검합니다:
     - `DOC-001`: `PATH`에서 `guard` 바이너리 탐색
     - `DOC-002`: `.agents/hooks.json` JSON 유효성 및 `guard hook` 등록 여부
     - `DOC-003`: `guard-policy.yaml` 로드 및 파싱 경고 여부
     - `DOC-004`: `.agents/rules/guard-workflow.md` 존재 및 `trigger: always_on` frontmatter 여부
     - `DOC-005`: 세션 디렉토리(`~/.guard/sessions`) 쓰기 권한 및 Stale PID 세션 수
     - `DOC-006`: 워크스페이스 경로의 symlink 일치 여부
   - `--fix`: 자동 복구 가능한 항목(`DOC-002` 누락, `DOC-003` 파일 미존재, `DOC-004` 누락/frontmatter 미포함, `DOC-005` 디렉토리 미존재/Stale 세션)을 즉시 수정합니다. 단, 깨진 JSON이나 문법 오류가 있는 기존 정책 파일은 덮어쓰지 않고 `ERROR`를 유지합니다.
   - `--json`: CI·스크립트 연동을 위해 stdout에 순수 JSON(`DoctorReport`)만 출력합니다.
3. **Launcher Bootstrap 통합 (SSOT)**
   - `internal/adapters/launcher/bootstrap.go`의 `EnsureWorkspaceHook`과 `EnsureWorkflowRule`이 기존 시그니처를 유지한 채 `internal/hub/setup`을 호출합니다.
   - `guard dev agy` 실행 시 자동 생성되는 `guard-workflow.md`에도 `trigger: always_on`이 기본 포함됩니다.
4. **Symlink 워크스페이스 경로 정규화**
   - `cmd/guard/main.go`의 `findWorkspaceRoot()`가 `filepath.EvalSymlinks()`로 canonical path를 반환해 symlink 경로 불일치를 방지합니다.
5. **다국어(i18n) 지원**
   - `internal/i18n/locales/ko.json`과 `en.json`에 `cli.help`, `cli.init.*`, `cli.doctor.*` 키가 1:1로 등록되어 있습니다.

---

## 2. 대상 패키지 맵

| 계층 | 패키지 | 핵심 테스트 대상 | 계약서 |
| :--- | :--- | :--- | :--- |
| **Setup Hub** | `guard/internal/hub/setup` | `DetectStack`, `ValidatePreset`, `MergeWorkspaceHook`, `InspectWorkspaceHook`, `RenderPolicyYAML`, `HasAlwaysOnFrontmatter`, `InjectAlwaysOnFrontmatter`, `EnsureWorkflowRule`, `RunInit`, `RunDoctor` | [06_init_automation_contract.md](../spec/06_init_automation_contract.md) |
| **Launcher** | `guard/internal/adapters/launcher` | `EnsureWorkspaceHook`, `EnsureWorkflowRule` 호환성 및 `trigger: always_on` 생성 | [03_adapters_cli_contract.md](../spec/03_adapters_cli_contract.md#2-launcher-adapter-guardinternaladapterslauncher) |
| **CLI** | `guard/cmd/guard` | `guard init`, `guard doctor`, `guard help`, symlink `WORKSPACE_ROOT` 처리 | [06_init_automation_contract.md](../spec/06_init_automation_contract.md#4-cli-계약-guard-init-guard-doctor) |
| **i18n** | `guard/internal/i18n` | `ko.json` / `en.json` 키 집합 및 포맷 동사 일치 | [03_adapters_cli_contract.md](../spec/03_adapters_cli_contract.md#4-i18n-guardinternali18n) |

---

## 3. 테스트용 공개 API 레퍼런스 (`guard/internal/hub/setup`)

### 3.1 상수 및 에러
```go
// 프리셋 상수
const (
    PresetAuto    = "auto"
    PresetGo      = "go"
    PresetNode    = "node"
    PresetPython  = "python"
    PresetGeneric = "generic"
)

// 관리 마커
const WorkflowManagedMarker = "<!-- guard-managed: v1 -->"

// Init 단계 상태
type StepStatus string
const (
    StatusCreated  StepStatus = "created"
    StatusMerged   StepStatus = "merged"
    StatusSkipped  StepStatus = "skipped"
    StatusPlanned  StepStatus = "planned"
    StatusBackedUp StepStatus = "backed_up"
)

// Doctor 심각도 및 체크 코드
type CheckSeverity string
const (
    SeverityOK    CheckSeverity = "OK"
    SeverityWarn  CheckSeverity = "WARN"
    SeverityError CheckSeverity = "ERROR"
    SeverityFixed CheckSeverity = "FIXED"
)

const (
    CodeBinaryPath   = "DOC-001"
    CodeHooksJSON    = "DOC-002"
    CodePolicyYAML   = "DOC-003"
    CodeWorkflowRule = "DOC-004"
    CodeSessionStore = "DOC-005"
    CodeSymlinkPath  = "DOC-006"
)

// Sentinel Errors (errors.Is로 검증)
var (
    ErrInvalidPreset    = errors.New("invalid preset")
    ErrInvalidMode      = errors.New("invalid mode")
    ErrInvalidHooksJSON = errors.New("invalid hooks.json")
)
```

### 3.2 DTO (입력 옵션 · 출력 결과 · JSON 스키마)
```go
type InitOptions struct {
    WorkspaceRoot string
    Preset        string // "", "auto", "go", "node", "python", "generic"
    Mode          string // "", "DEV", "PLAN", "TEST", "ALL"
    DryRun        bool
    Force         bool
    NoPolicy      bool
    NoRules       bool
    NoHooks       bool
}

type StepResult struct {
    Target     string     `json:"target"`
    Status     StepStatus `json:"status"`
    Detail     string     `json:"detail,omitempty"`
    BackupPath string     `json:"backup_path,omitempty"`
}

type InitResult struct {
    WorkspaceRoot  string       `json:"workspace_root"`
    DetectedPreset string       `json:"detected_preset"`
    DetectedMarker string       `json:"detected_marker,omitempty"`
    DryRun         bool         `json:"dry_run"`
    Steps          []StepResult `json:"steps"`
}

type HookMergeResult struct {
    Path          string
    Status        StepStatus
    PreservedKeys []string
}

type DoctorOptions struct {
    WorkspaceRoot string
    SessionMgr    *session.Manager                  // nil이면 session.NewManager("")
    Fix           bool
    LookPath      func(file string) (string, error) // nil이면 exec.LookPath (테스트 DI용)
}

type CheckResult struct {
    Code     string        `json:"code"`
    Name     string        `json:"name"`
    Severity CheckSeverity `json:"severity"`
    Message  string        `json:"message"`
    Fixable  bool          `json:"fixable"`
    Fixed    bool          `json:"fixed,omitempty"`
}

type DoctorReport struct {
    WorkspaceRoot string        `json:"workspace_root"`
    Checks        []CheckResult `json:"checks"`
    ErrorCount    int           `json:"error_count"`
    WarnCount     int           `json:"warn_count"`
    FixedCount    int           `json:"fixed_count"`
}
```

### 3.3 공개 함수 및 메서드 시그니처
```go
// 스택 감지 및 검증
func DetectStack(workspaceRoot string) (preset string, marker string)
func ValidatePreset(preset string) error

// .agents/hooks.json 검사 및 Safe Merge
func InspectWorkspaceHook(workspaceRoot string) (exists bool, validJSON bool, hasGuardHook bool, err error)
func MergeWorkspaceHook(workspaceRoot string, dryRun bool) (HookMergeResult, error)

// 정책 및 워크플로우 룰 템플릿
func RenderPolicyYAML(preset string) (string, error)
func WorkflowRuleContent() string
func HasAlwaysOnFrontmatter(content string) bool
func InjectAlwaysOnFrontmatter(content string) string
func EnsureWorkflowRule(workspaceRoot string) (rulePath string, created bool, err error)

// 오케스트레이터 및 진단 엔진
func RunInit(opts InitOptions) (InitResult, error)
func RunDoctor(opts DoctorOptions) DoctorReport
func (r DoctorReport) ExitCode() int
```

### 3.4 Launcher 호환 시그니처 (`guard/internal/adapters/launcher`)
```go
func EnsureWorkspaceHook(workspace string) (hookPath string, updated bool, err error)
func EnsureWorkflowRule(workspace string) (rulePath string, created bool, err error)
```

---

## 4. CLI 인터페이스 및 Exit Code

| 명령 | 플래그 / 동작 | Exit Code |
| :--- | :--- | :---: |
| `guard init` | `--preset`, `--mode`, `--dry-run`, `--force`, `--no-policy`, `--no-rules`, `--no-hooks`. 알 수 없는 플래그나 positional args 입력 시 즉시 실패(Fail-Fast) | 성공 `0` / 실패 `1` |
| `guard doctor` | `--fix`, `--json`. `--json` 시 stdout에 `DoctorReport` JSON만 출력(일반 배너 출력 금지). 알 수 없는 플래그나 positional args 입력 시 즉시 실패 | `ErrorCount == 0`: `0`<br>`ErrorCount > 0`: `1` |

---

## 5. 격리 테스트 환경 구성 가이드

### 5.1 Go Black-box 테스트 (`package setup_test` / `test/e2e`)
- 파일 시스템은 반드시 `t.TempDir()`을 사용합니다.
- `RunDoctor` 테스트 시 `SessionMgr: session.NewManager(t.TempDir())`와 `LookPath` mock 함수를 주입해 호스트 환경(`~/.guard/sessions`, 실제 `PATH`)과 완전히 분리합니다.
  ```go
  report := setup.RunDoctor(setup.DoctorOptions{
      WorkspaceRoot: wsDir,
      SessionMgr:    session.NewManager(filepath.Join(tmp, "sessions")),
      Fix:           true,
      LookPath: func(file string) (string, error) {
          return "/usr/local/bin/guard", nil
      },
  })
  ```

### 5.2 CLI 통합 / E2E 테스트
```bash
# 바이너리 빌드
go build -o ./guard ./cmd/guard

# 임시 디렉토리 격리 실행
export TEST_HOME=$(mktemp -d)
export TEST_WS=$(mktemp -d)
touch "$TEST_WS/go.mod"

HOME="$TEST_HOME" WORKSPACE_ROOT="$TEST_WS" ./guard init --dry-run
HOME="$TEST_HOME" WORKSPACE_ROOT="$TEST_WS" ./guard init
HOME="$TEST_HOME" WORKSPACE_ROOT="$TEST_WS" PATH="$(pwd):$PATH" ./guard doctor --json
```

---

## 6. 최상위 수용 기준 (Acceptance Criteria)

세부 테스트 케이스는 QA에서 [06_init_automation_contract.md](../spec/06_init_automation_contract.md)를 기준으로 독립 설계합니다.

1. **스택 자동 감지 & 정책 유효성**: 마커 파일 우선순위(`go.mod` > `package.json` > `pyproject.toml` > `requirements.txt` > `generic`)대로 프리셋이 선택되며, 4종 프리셋 YAML 모두 `policy.LoadFile()`에서 경고·오류 없이 로드되어야 한다.
2. **Safe Merge & 깨진 JSON 보호**: `.agents/hooks.json`의 기존 타 키(`obsidian-chatlog` 등)가 100% 보존되고, 문법이 깨진 JSON 파일은 `init`과 `doctor --fix` 모두에서 1바이트도 수정되지 않고 에러를 리턴해야 한다.
3. **Idempotent / Dry-Run / Force Backup**: `init` 2회 실행 시 2회차 전 항목 `skipped` + 파일 내용 무변경, `--dry-run` 시 파일·디렉토리 미생성, `--force` 시 `.bak` 백업 생성이 보장되어야 한다.
4. **Always-On 룰 주입 및 비파괴 복구 (`DOC-004`)**: `init` 및 `launcher.EnsureWorkflowRule` 생성 파일에 `trigger: always_on`이 포함되어야 하며, `trigger: always_on`만 빠진 기존 사용자 파일에 `doctor --fix` 실행 시 기존 본문을 보존한 채 frontmatter만 삽입되어야 한다.
5. **Doctor 진단 정확성 & JSON 출력**: `DOC-001`~`DOC-006`의 `OK`/`WARN`/`ERROR`/`FIXED` 판정과 `ExitCode()`(`ERROR >= 1`이면 `1`, `WARN`만 있으면 `0`)가 정확해야 하며, `guard doctor --json`의 stdout은 순수 유효 JSON이어야 한다.
6. **Symlink 정규화 & i18n 대칭성**: symlink 경로를 통한 CLI 실행 시 canonical path로 정규화되어야 하며, `ko.json`/`en.json` 키와 포맷 동사가 100% 일치해야 한다.
