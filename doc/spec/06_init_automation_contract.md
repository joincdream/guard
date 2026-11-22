# [06_init_automation_contract.md] 초기 설정 자동화 및 자가 진단 계약서 (`guard init` / `guard doctor`)

> **대상 독자**: QA (블랙박스 테스트 작성 기준)  
> **대상 패키지**: `guard/internal/hub/setup`, `guard/internal/adapters/launcher`, `guard/cmd/guard`, `guard/internal/i18n`  
> **관련 설계서**: [09_init_automation_design.md](../09_init_automation_design.md), [init_automation_dev_plan.md](../dev/init_automation_dev_plan.md)

---

## 1. Setup 패키지 공개 상수 · 에러 · DTO (`guard/internal/hub/setup`)

```go
const (
    PresetAuto    = "auto"
    PresetGo      = "go"
    PresetNode    = "node"
    PresetPython  = "python"
    PresetGeneric = "generic"
)

const WorkflowManagedMarker = "<!-- guard-managed: v1 -->"

type StepStatus string
const (
    StatusCreated  StepStatus = "created"
    StatusMerged   StepStatus = "merged"
    StatusSkipped  StepStatus = "skipped"
    StatusPlanned  StepStatus = "planned"
    StatusBackedUp StepStatus = "backed_up"
)

type CheckSeverity string
const (
    SeverityOK    CheckSeverity = "OK"
    SeverityWarn  CheckSeverity = "WARN"
    SeverityError CheckSeverity = "ERROR"
    SeverityFixed CheckSeverity = "FIXED"
)

const (
    CodeBinaryPath    = "DOC-001"
    CodeHooksJSON     = "DOC-002"
    CodePolicyYAML    = "DOC-003"
    CodeWorkflowRule  = "DOC-004"
    CodeSessionStore  = "DOC-005"
    CodeSymlinkPath   = "DOC-006"
)

var (
    ErrInvalidPreset    = errors.New("invalid preset")
    ErrInvalidMode      = errors.New("invalid mode")
    ErrInvalidHooksJSON = errors.New("invalid hooks.json")
)

type InitOptions struct {
    WorkspaceRoot string
    Preset        string // "", "auto", "go", "node", "python", "generic"
    Mode          string // "", "DEV", "PLAN", "TEST", "ALL" (대소문자 무관)
    DryRun        bool
    Force         bool
    NoPolicy      bool
    NoRules       bool
    NoHooks       bool
}

type StepResult struct {
    Target     string     `json:"target"`                // 예: ".agents/hooks.json"
    Status     StepStatus `json:"status"`
    Detail     string     `json:"detail,omitempty"`      // 예: "preset: go", "preserved: obsidian-chatlog"
    BackupPath string     `json:"backup_path,omitempty"` // --force로 백업 시 ".bak" 경로
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
    Status        StepStatus // created | merged | skipped | planned
    PreservedKeys []string   // 기존에 존재하던 타 최상위 키 목록 (정렬)
}

type DoctorOptions struct {
    WorkspaceRoot string
    SessionMgr    *session.Manager
    Fix           bool
    LookPath      func(file string) (string, error) // nil이면 exec.LookPath 사용
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

---

## 2. 공개 함수 및 메서드 계약 (`guard/internal/hub/setup`)

### 2.1 스택 감지 · 템플릿 · 훅 병합
| 시그니처 | 동작 계약 |
| :--- | :--- |
| `DetectStack(workspaceRoot string) (preset string, marker string)` | 프로젝트 루트에서 `go.mod` → `package.json` → `pyproject.toml` → `requirements.txt` 순으로 탐지하여 `go`/`node`/`python` 반환. 미감지 시 `("generic", "")` 반환 |
| `ValidatePreset(preset string) error` | `""`, `auto`, `go`, `node`, `python`, `generic`만 허용 (대소문자 무관). 그 외는 `ErrInvalidPreset` 반환 |
| `MergeWorkspaceHook(workspaceRoot string, dryRun bool) (HookMergeResult, error)` | `.agents/hooks.json` 미존재 시 기본 JSON 생성(`StatusCreated`), 존재 시 JSON 파싱 후 최상위 `"guard"` 키가 없거나 `PreToolUse`에 `"command": "guard hook"`이 없으면 추가(`StatusMerged`), 이미 등록되어 있으면 `StatusSkipped`. 타 최상위 키(예: `obsidian-chatlog`)는 구조/값 손실 없이 100% 보존. 깨진 JSON이면 원본을 수정하지 않고 `ErrInvalidHooksJSON` 래핑 에러 반환. `dryRun == true`이면 파일 생성/수정 없이 `StatusPlanned` 또는 `StatusSkipped`만 반환. 실제 쓰기는 2-space 들여쓰기 + `.tmp` → `os.Rename` 원자적 쓰기 수행 |
| `InspectWorkspaceHook(workspaceRoot string) (exists bool, validJSON bool, hasGuardHook bool, err error)` | `.agents/hooks.json` 존재 여부, JSON 유효성, `guard.PreToolUse` 내 `guard hook` 등록 여부를 비파괴 조회 |
| `RenderPolicyYAML(preset string) (string, error)` | `go`/`node`/`python`/`generic` 프리셋별 `guard-policy.yaml` 문자열 반환 (`policy.LoadFile` 파싱 시 경고·오류 없이 통과해야 함). 미지원 프리셋은 `ErrInvalidPreset` |
| `WorkflowRuleContent() string` | 상단에 `---\ntrigger: always_on\n---` YAML frontmatter와 `<!-- guard-managed: v1 -->` 마커, `guard handoff` CLI 치트시트(`inbox`, `status`, `ack`, `send`, `approve` 금지 경계)가 포함된 마크다운 반환 |
| `HasAlwaysOnFrontmatter(content string) bool` | 문서 최상단 YAML frontmatter(`---` ... `---`) 내에 `trigger: always_on`이 정의되어 있으면 `true` |
| `InjectAlwaysOnFrontmatter(content string) string` | `HasAlwaysOnFrontmatter`가 `true`이면 원본 그대로 반환. 상단 frontmatter가 없으면 `---\ntrigger: always_on\n---\n\n`을 앞에 추가. 기존 frontmatter가 있으나 `trigger`만 없으면 기존 frontmatter 내부에 `trigger: always_on`을 삽입하며 본문을 100% 보존 |
| `EnsureWorkflowRule(workspaceRoot string) (rulePath string, created bool, err error)` | `<workspaceRoot>/.agents/rules/guard-workflow.md` 미존재 시 `WorkflowRuleContent()`로 생성(`created: true`), 이미 존재 시 변경 없이 보존(`created: false`) |

### 2.2 초기화(`RunInit`) 및 자가 진단(`RunDoctor`)
| 시그니처 | 동작 계약 |
| :--- | :--- |
| `RunInit(opts InitOptions) (InitResult, error)` | 1. `Preset`, `Mode` 유효성 검증 (잘못된 값이면 파일 변경 없이 즉시 에러 반환)<br>2. `--no-hooks`가 아니면 `MergeWorkspaceHook` 수행<br>3. `--no-policy`가 아니면 `guard-policy.yaml` 생성 (기존 파일 존재 시 `StatusSkipped`, `Force == true`이면 `guard-policy.yaml.bak` 백업 후 재생성하여 `StatusBackedUp`)<br>4. `--no-rules`가 아니면 `.agents/rules/guard-workflow.md` 생성 (기존 파일 존재 시 `StatusSkipped`, `Force == true`이면 `.bak` 백업 후 재생성)<br>5. `.agents/current_mode` 미존재 시 정규화된 모드값(`DEV`/`PLAN`/`TEST`/`ALL`, 기본 `DEV`) 기록, 존재 시 `StatusSkipped`<br>6. `DryRun == true`이면 어떠한 파일·디렉토리도 생성·수정하지 않고 계획만 반환 |
| `RunDoctor(opts DoctorOptions) DoctorReport` | `DOC-001` ~ `DOC-006` 6개 항목을 순서대로 검사하여 `DoctorReport` 반환 (`ErrorCount`, `WarnCount`, `FixedCount` 정확히 집계) |
| `(r DoctorReport) ExitCode() int` | `r.ErrorCount > 0`이면 `1`, 아니면 `0` (Warning만 있는 경우 `0`) |

### 2.3 `guard doctor` 진단 매트릭스 계약 (`DOC-001` ~ `DOC-006`)
| 코드 | 검사 대상 | 이상 시 심각도 | `Fix: true` 자동 복구 계약 |
| :--- | :--- | :---: | :--- |
| `DOC-001` | `LookPath("guard")` | `ERROR` | 복구 불가 (`Fixable: false`), 안내 메시지 반환 |
| `DOC-002` | `.agents/hooks.json` 유효성 및 `guard hook` 등록 | `ERROR` | `Fixable: true` — 미존재 또는 `guard hook` 누락 시 `MergeWorkspaceHook`으로 복구 후 `SeverityFixed`(`Fixed: true`). 단, JSON 구문이 깨진 파일은 덮어쓰지 않고 `ERROR` 유지 |
| `DOC-003` | `policy.LoadDetailed(workspaceRoot)` | 파싱 오류: `ERROR`<br>builtin fallback: `WARN` | 정책 파일 미존재(`builtin`)일 때만 자동 프리셋 생성 후 `SeverityFixed`. 기존 정책 파일의 문법 오류는 덮어쓰지 않고 `ERROR` 유지 |
| `DOC-004` | `.agents/rules/guard-workflow.md` 존재 및 `trigger: always_on` | `WARN` | `Fixable: true` — 파일 미존재 시 생성, frontmatter 누락 시 `InjectAlwaysOnFrontmatter`로 기존 본문을 보존하며 삽입 후 `SeverityFixed` |
| `DOC-005` | `SessionMgr.Dir()` 쓰기 권한 및 Stale PID 세션 | 디렉토리 미존재 / Stale 세션 존재: `WARN`<br>접근·쓰기 불가: `ERROR` | `Fixable: true` — 디렉토리 미존재 시 생성, Stale PID 세션 존재 시 `CleanStaleSessions()` 실행 후 `SeverityFixed` |
| `DOC-006` | `WorkspaceRoot` vs `filepath.EvalSymlinks(WorkspaceRoot)` 일치 여부 | `WARN` | 복구 불가 (`Fixable: false`), 심볼릭 링크 경로 불일치 안내 반환 |

---

## 3. Launcher Bootstrap 호환성 계약 (`guard/internal/adapters/launcher`)

- `EnsureWorkspaceHook(workspace string) (hookPath string, updated bool, err error)`:
  - 기존 시그니처와 반환 의미를 유지하되, 내부적으로 `setup.MergeWorkspaceHook`을 사용하여 원자적 쓰기 및 `PreToolUse` 내 `guard hook` 정밀 검증을 보장해야 합니다.
- `EnsureWorkflowRule(workspace string) (rulePath string, created bool, err error)`:
  - 기존 시그니처와 반환 의미를 유지하되, 신규 생성되는 `guard-workflow.md`에는 반드시 `trigger: always_on` frontmatter가 포함되어야 합니다. 이미 존재하는 파일은 덮어쓰지 않습니다.

---

## 4. CLI 계약 (`guard init`, `guard doctor`)

| 명령 | 동작 계약 | 종료 코드 |
| :--- | :--- | :---: |
| `guard init [--preset <auto\|go\|node\|python\|generic>] [--mode <DEV\|PLAN\|TEST\|ALL>] [--dry-run] [--force] [--no-policy] [--no-rules] [--no-hooks]` | 워크스페이스 초기 설정 실행 및 항목별 결과 출력. 잘못된 플래그·예상치 못한 positional args·잘못된 프리셋/모드 또는 깨진 `hooks.json` 시 stderr 출력 (Fail-Fast) | 성공 `0` / 실패 `1` |
| `guard doctor [--fix] [--json]` | `DOC-001`~`DOC-006` 진단 실행. 잘못된 플래그·예상치 못한 positional args 시 Fail-Fast(`1`). `--json` 시 stdout에 유효한 `DoctorReport` JSON만 출력(부가 배너 혼합 금지). `--fix` 시 복구 가능 항목 수정 후 최종 상태 보고 | `ErrorCount == 0`: `0`<br>`ErrorCount > 0`: `1` |
| `guard help` / `guard --help` | 도움말 출력에 `guard init` 및 `guard doctor` 항목 포함 | `0` |

---

## 5. 핵심 불변 계약 (QA 검증 포인트)

1. **비파괴 JSON 병합 및 손상 방지**:
   - `.agents/hooks.json`에 `obsidian-chatlog` 등 타 최상위 키가 존재할 때 `guard init` 실행 후 타 키의 값과 구조가 100% 보존되어야 합니다.
   - 문법이 깨진 `.agents/hooks.json`(예: `{invalid`)에 대해 `guard init` 또는 `guard doctor --fix`를 실행해도 원본 파일의 바이트가 전혀 변경되지 않고 오류를 반환해야 합니다.
2. **멱등성 (Idempotency)**:
   - `guard init`을 2회 연속 실행했을 때, 2회차의 모든 단계가 `StatusSkipped`이고 생성된 4개 파일의 바이트 내용이 1회차 결과와 완전히 동일해야 합니다.
3. **`--dry-run` 무변경 보장**:
   - 빈 디렉토리 또는 기존 파일이 있는 디렉토리에서 `guard init --dry-run`을 실행했을 때, 파일 시스템에 어떠한 파일·디렉토리(`.agents/` 포함)도 생성되거나 수정되지 않아야 합니다.
4. **`--force` 백업 무손실**:
   - 커스텀 `guard-policy.yaml`이 있는 상태에서 `guard init --force` 실행 시 기존 내용이 `guard-policy.yaml.bak`에 그대로 보존되고, 새 `guard-policy.yaml`이 생성되어야 합니다.
5. **스택 감지 우선순위 및 정책 유효성**:
   - `go.mod`와 `package.json`이 공존하면 우선순위에 따라 `go` 프리셋이 선택되어야 합니다.
   - `go`, `node`, `python`, `generic` 4종 프리셋으로 생성된 모든 `guard-policy.yaml`은 `policy.LoadFile()`에서 오류·경고 없이 로드되고 1개 이상의 규칙을 가져야 합니다.
6. **`DOC-004` 비파괴 자동 복구**:
   - 사용자가 직접 작성한 본문이 있고 `trigger: always_on`만 빠진 `.agents/rules/guard-workflow.md`에 대해 `guard doctor`는 `DOC-004 WARN`을 보고하고, `guard doctor --fix` 실행 후에는 기존 본문이 100% 유지된 채 상단에 `trigger: always_on`이 삽입되며 재진단 시 `DOC-004 OK`가 되어야 합니다.
7. **심볼릭 링크 정규화 및 회귀 방지**:
   - 심볼릭 링크 경로를 통해 실행된 `guard status` 및 핸드오프 라우팅이 실제 경로와 동일하게 정규화되어야 하며, 기존 `test/e2e/handoff_*` 시나리오에 회귀가 없어야 합니다.
8. **i18n 키 대칭성**:
   - `internal/i18n/locales/ko.json`과 `en.json`의 모든 `cli.init.*`, `cli.doctor.*` 키 집합 및 포맷 동사(`%s`, `%d`, `%v`) 개수가 100% 일치해야 합니다.
