# [03_adapters_cli_contract.md] Adapters (Hook · Launcher · TUI), i18n 및 CLI 계약서

> **대상 독자**: QA (블랙박스 테스트 작성 기준)
> **대상 패키지**: `guard/internal/adapters/hook`, `guard/internal/adapters/launcher`, `guard/internal/adapters/tui`, `guard/internal/i18n`, `guard/cmd/guard`

---

## 1. Hook Adapter (`guard/internal/adapters/hook`)

### 1.1 입출력 JSON DTO
```go
type Input struct {
    SessionID      string                 `json:"sessionId"`
    SessionIDSnake string                 `json:"session_id"`
    ConversationID string                 `json:"conversationId"`
    CWD            string                 `json:"cwd"`
    WorkspacePaths []string               `json:"workspacePaths"`
    ToolCall       struct {
        Name string                 `json:"name"`
        Args map[string]interface{} `json:"args"`
    } `json:"toolCall"`
    ToolName    string                 `json:"tool_name"`
    ToolArgs    map[string]interface{} `json:"tool_args"`
    ToolInput   map[string]interface{} `json:"tool_input"`
    Path        string                 `json:"Path"`
    PathLower   string                 `json:"path"`
    CommandLine string                 `json:"CommandLine"`
    CmdLower    string                 `json:"command_line"`
}

type Output struct {
    Decision string `json:"decision"`         // "allow" 또는 "deny"
    Reason   string `json:"reason,omitempty"` // deny 시 차단 사유
}
```

### 1.2 공개 시그니처 및 동작 계약
- `NewHandler(sessionMgr *session.Manager, workspaceRoot string, stdout, stderr io.Writer) *Handler`
- `(h *Handler) Handle(r io.Reader) error`
- **Fail-open 계약**: 빈 입력 또는 잘못된 JSON 입력 시 에러 없이 `{"decision":"allow"}` 출력.
- **세션 및 모드 해석 우선순위**:
  1. 후보 세션 ID(`GUARD_SESSION_ID` → 페이로드 `sessionId`/`session_id`/`conversationId` → `SESSION_ID` → `CONVERSATION_ID`) 중 **`sessionMgr`에 실제 존재하는 첫 번째 세션**의 모드를 최우선 적용.
  2. 일치하는 세션이 없으면 `policy.ResolveWorkspaceMode(effectiveWorkspace)` (`AGENT_MODE` → `AGENT_ROLE` → `.agents/current_mode` → `DEV`)로 폴백.
- **이벤트 기록 계약**: `sessionMgr`에 이미 존재하는 추적 세션(`hasSession == true`)에만 `AppendEvent` 수행. 존재하지 않는 세션에 대해 `PID: 0` 세션을 임의 생성하지 않음.
- **차단 시 출력**: `stderr`에 `Reason` 1줄 출력 + `stdout`에 `{"decision":"deny","reason":"..."}` JSON 출력.
- **진단 로그**: `GUARD_DEBUG=1` 설정 시에만 `stderr`에 `[guard:debug]` 로그 출력 (미설정 시 무출력).

---

## 2. Launcher Adapter (`guard/internal/adapters/launcher`)

### 2.1 공개 타입 및 함수
```go
type BannerInfo struct {
    Mode              string
    SessionID         string
    Workspace         string
    PolicySource      string
    PolicyWarnings    []string
    CmdName           string
    SkillPath         string
    SkillCreated      bool
    HookPath          string
    HookUpdated       bool
    HasHook           bool
    RulePath          string
    RuleCreated       bool
    DeliveredHandoffs int
    InboxPath         string
}
```
- `New(sessionMgr *session.Manager, stdout, stderr io.Writer) *Launcher`
- `(l *Launcher) WithHandoffManager(m *handoff.Manager) *Launcher` — 테스트 또는 커스텀 핸드오프 매니저 주입
- `(l *Launcher) Run(mode, workspace string, cmdArgs []string) int`
  - 빈 `cmdArgs` 시 `stderr` 오류 출력 후 종료 코드 `1`.
  - 자식 프로세스 실행 시 `AGENT_MODE`, `GUARD_SESSION_ID`, `GUARD_INBOX_PATH` 환경변수 주입, 오프라인 대기 핸드오프(`ConsumePendingForMode`) 자동 소비 및 조건부 `AllowedWritePaths` 초기 바인딩, 실제 자식 PID로 세션 생성, 종료 시 `defer Delete(sessionID)` 및 `.inbox` 파일 정리.
  - 워크스페이스 부트스트랩(`EnsureWorkspaceHook`, `EnsureWorkflowRule`)은 일반 셸 명령 실행 시 워크스페이스 오염을 방지하기 위해 `filepath.Base(cmdArgs[0]) == "agy"`일 때만 자동 실행.
  - `PolicyWarnings`가 있으면 배너에 `⚠️  [Policy Warning]` 출력.
  - `stdout`이 TTY(`*os.File` + `os.ModeCharDevice`)일 때만 ANSI OSC 타이틀 시퀀스(`\033]0;...\007`) 출력 (버퍼/파이프 출력 오염 금지).
- `PrintBanner(w io.Writer, info BannerInfo)`
- `EnsureAgentSkill(homeDir string) (skillPath string, created bool, err error)` — 미존재 시에만 생성, 기존 파일 보존.
- `EnsureWorkspaceHook(workspace string) (hookPath string, updated bool, err error)` — `setup.MergeWorkspaceHook`을 호출해 `<workspace>/.agents/hooks.json`에 `guard` PreToolUse hook을 Atomic Write로 merge(기존 `obsidian-chatlog` 등 타 키 보존, 손상된 JSON은 덮어쓰지 않고 에러 반환).
- `EnsureWorkflowRule(workspace string) (rulePath string, created bool, err error)` — `setup.EnsureWorkflowRule`을 호출해 `<workspace>/.agents/rules/guard-workflow.md` 미존재 시 `trigger: always_on` YAML frontmatter 포함 템플릿으로 생성, 기존 파일 보존.
- `HasWorkspaceHook(workspace string) bool`

---

## 3. TUI Control Tower (`guard/internal/adapters/tui`)

### 3.1 공개 API
- `NewModel(sessMgr *session.Manager) Model`
- `NewModelWithHandoff(sessMgr *session.Manager, handoffMgr *handoff.Manager, workspaceRoot string) Model`
- `Start(sessMgr *session.Manager) error`
- `StartWithHandoff(sessMgr *session.Manager, handoffMgr *handoff.Manager, workspaceRoot string) error`

### 3.2 화면 및 인터랙션 계약
- **활성 세션 테이블 (5컬럼)**: `ID | 모드 | 워크스페이스 | 최근 활동 | 차단` (`PID`/`상태` 컬럼 없음, `lipgloss.Width` 기반 1줄 정렬).
- **주기 갱신**: 0.5초 틱마다 `CleanAndList()` 및 핸드오프 큐 조회(`reloadHandoffs`) 수행.
- **키 바인딩**:
  - `↑`/`k`, `↓`/`j`: 세션 커서 이동
  - `d`/`D`, `t`/`T`, `p`/`P`, `a`/`A`: 선택 세션 모드 즉시 전환 (`DEV`/`TEST`/`PLAN`/`ALL`)
  - `c`/`C`: 선택 세션 차단 카운트 0 리셋
  - `Space`: 전체 이벤트 로그 모달 토글 (`Esc` 닫기)
  - `h`/`H`: 하단 패널 `최근 이벤트 로그 ↔ Handoff Queue` 전환
  - 핸드오프 요약 모달 활성 시: `Enter`(승인·라우팅), `v`/`V`(참조 문서 256KB 상한 스크롤 뷰어), `Esc`(반려)
  - `?`: 도움말 오버레이 토글 (`Esc` 닫기)
  - `r`/`R`: 즉시 새로고침
  - `q`/`Q`/`Ctrl+C`: 종료

---

## 4. i18n (`guard/internal/i18n`)

### 4.1 공개 API
- `const DefaultLang = "en"`
- `Init()`, `SetLang(lang string)`, `Lang() string`, `SupportedLangs() []string`
- `DetectLocale() string`, `DetectLocaleFromLookup(getenv func(string) string) string`, `ParseLocale(raw string) string`
- `T(key string, args ...any) string`
- `RelativeTime(t time.Time) string`

### 4.2 동작 계약
- 감지 우선순위: `GUARD_LANG` → `LC_ALL` → `LC_MESSAGES` → `LANG` → `"en"`.
- `C`, `POSIX`, 미지원 언어는 `"en"` 폴백.
- 키 조회 폴백: `활성 언어` → `"en"` → `키 원문`.
- 내장 카탈로그(`en.json`, `ko.json`)는 키 집합 및 `fmt` 포맷 동사 개수가 100% 일치해야 함.

---

## 5. CLI (`guard/cmd/guard`)

| 명령 | 동작 계약 | 종료 코드 |
| :--- | :--- | :---: |
| `guard` (stdin 파이프) / `guard hook` | PreToolUse 훅 실행 | `0` (정상) / `1` (I/O 오류) |
| `guard` (TTY) / `guard help` / `-h` / `--help` | 도움말 출력 (`guard init`, `guard doctor` 포함) | `0` |
| `guard init [flags]` | 워크스페이스 스택 감지 및 hook·정책·룰·초기 모드 초기화 ([06_init_automation_contract.md](./06_init_automation_contract.md)) | `0` / `1` |
| `guard doctor [--fix] [--json]` | `DOC-001`~`DOC-006` 환경/설정 진단 및 자동 복구 ([06_init_automation_contract.md](./06_init_automation_contract.md)) | `0` / `1` |
| `guard dev\|test\|plan\|all` | `.agents/current_mode` 저장 및 활성 세션 모드 갱신 | `0` / `1` |
| `guard dev\|test\|plan\|all <cmd...>` / `guard run <mode> <cmd...>` | 지정 모드로 명령 격리 실행 | 자식 프로세스 종료 코드 |
| `guard <executable...>` | 워크스페이스 기본 모드로 실행 | 자식 프로세스 종료 코드 |
| `guard status` | 워크스페이스·모드·정책 출처(파싱 경고 포함)·활성 세션 요약 출력 | `0` |
| `guard top` / `guard ui` | TUI 관제탑 실행 (핸드오프 모달·큐 포함) | `0` / `1` |
| `guard handoff <send\|inbox\|approve\|reject\|ack>` | 범용 핸드오프 티켓 발행·조회·승인·반려·완료 ([05_handoff_protocol_contract.md](./05_handoff_protocol_contract.md)) | `0` / `1` |
| `guard clean [--all\|-a]` | 종료/무효 세션 정리 (`--all`: 전체 삭제) | `0` / `1` |
| `guard reset <sessionId>` | 특정 세션 차단 카운트 0 초기화 | `0` / `1` |
| `guard doc` / `guard sample` | 정책 스펙 및 Few-Shot 샘플 출력 | `0` |
| 알 수 없는 서브커맨드 | `stderr` 오류 + 도움말 출력 | `1` |
