# [07_refactoring_plan.md] Guard 프로덕트 품질 점검 및 리팩토링 계획서

## 1. 목적 및 배경

`guard`는 Antigravity CLI(`agy`)의 `PreToolUse` 훅, 다중 프로세스 세션 허브, 실시간 TUI 관제탑(`guard top`)을 단일 Go 바이너리로 제공하는 보안 가드레일 도구입니다. 본 문서는 **간결하고 운영·유지보수가 명확한 프로덕트 레벨 코드**를 목표로, 품질 점검 결과와 구체적인 리팩토링 항목을 정리합니다.

---

## 2. 품질 점검 카테고리 개요

1. **보안·정확성** (Guardrail Core & Hook) — **점검 완료**
2. **동시성·데이터 무결성** (Session Hub & Multi-Process I/O) — **점검 완료**
3. **에러 처리·운영 관측성** (Operability) — **점검 완료**
4. **코드 간결성·유지보수성** (Architecture & Maintainability) — **점검 완료**
5. **테스트·배포 완성도** (Verification & Release) — *예정*

---

## 3. [카테고리 1] 보안 · 정확성 점검 결과

| 심각도 | 대상 파일 | 현황 및 문제점 | 리팩토링 목표 |
| :---: | :--- | :--- | :--- |
| **높음** | `internal/core/matcher.go` (L94-L110) | `normalizePath`가 `./`와 `//`만 정리하고 **`..` 경로 순회를 정규화(`path.Clean`)하지 않음**. 예: `doc/../src/internal/app.go`로 패턴 매칭 우회 가능 | `path.Clean` 적용 (및 가능한 경우 워크스페이스 기준 상대 경로 정규화) |
| **높음** | `internal/core/command.go` (L8-L18, L209-L271) | **셸 우회 도구 누락**:<br/>• 읽기(`fileViewers`): `grep`, `rg`, `awk`, `sed`(조회), `jq`, `base64`, `hexdump`, `xxd`, `git grep` 누락<br/>• 쓰기(`inspectWriteSegment`): `cp`, `mv`, `rm`, `patch`, `dd`, `truncate` 누락<br/>• 명령 치환(`$(...)`, `` `...` ``) 내부 미검사 | 읽기/쓰기 명령어 집합 보강 및 `$(...)`·백틱 내부 재귀 검사 추가 |
| **중간** | `internal/hub/session/manager.go` (L181, L271-L275) | `sanitizeID`가 선행 `.`을 제거하지 않아 `.`으로 시작하는 ID 생성 시 `List()`의 숨김 파일 필터(`strings.HasPrefix(name, ".")`)에 걸려 관제탑에서 은닉됨 | `sanitizeID`에서 선행 `.`도 치환/제거 |
| **중간** | `internal/adapters/hook/handler.go` (L136-L144) | 런처 세션이 없는 폴백 경로에서 `.agents/current_mode`를 신뢰하는데, 에이전트가 해당 파일을 `ALL`로 직접 수정하는 행위는 기본 정책에서 차단되지 않음 | 내장 정책 또는 훅에서 `.agents/current_mode`, `.agents/hooks.json` 쓰기 보호 검토 |

---

## 4. [카테고리 2] 동시성 · 데이터 무결성 점검 결과

### 4.1 양호한 점 (유지)
- `internal/adapters/launcher/launcher.go`의 `doneCh` 기반 고루틴 종료, 시그널 정리(`signal.Stop`), 세션 정리(`defer Delete`)는 누수 없이 안전하게 구현됨.
- `internal/i18n/i18n.go`의 `sync.RWMutex` 동시 접근 보호 완료.

### 4.2 리팩토링 대상 항목

| 심각도 | 대상 파일 | 현황 및 문제점 | 리팩토링 목표 |
| :---: | :--- | :--- | :--- |
| **높음** | `internal/hub/session/manager.go` (L209-L261) | **세션 JSON Lost Update (경합)**: `UpdateMode`, `ResetViolations`, `AppendEvent`가 파일 락 없이 `Get()` → 수정 → `Save()` 수행.<br/>• 병렬 도구 호출 시 동시 `AppendEvent`로 이벤트/차단 수 유실<br/>• `guard top` 모드 전환 직후 훅의 `AppendEvent`가 구버전 `Mode`로 덮어써 **모드 변경이 롤백**될 수 있음 | 세션별 락 파일(`<id>.lock`)에 `syscall.Flock` 적용해 Read-Modify-Write 전체 구간 직렬화 |
| **중간** | `internal/adapters/launcher/launcher.go` (L87-L148, L190-L195) | `setTerminalTitle`이 **TTY 여부를 확인하지 않고 `l.stdout`에 ANSI OSC 시퀀스 출력**. 파이프/파일 리다이렉션 시 출력 오염 발생 가능하며, 자식 프로세스(`agy`) 출력과 동시 쓰기 경합 가능성 존재 | `l.stdout`이 TTY(`os.ModeCharDevice`)일 때만 출력하도록 가드 추가 |

---

## 5. [카테고리 3] 에러 처리 · 운영 관측성 점검 결과

| 심각도 | 대상 파일 | 현황 및 문제점 | 리팩토링 목표 |
| :---: | :--- | :--- | :--- |
| **높음** | `internal/hub/policy/loader.go` (L49-L55) | `guard-policy.yaml`에 YAML 문법 오류가 있어도 에러를 무시하고 `builtin` 정책으로 **무음 폴백**됨. 운영 시 사용자 정책 미적용을 인지하지 못함 | 파싱 실패한 후보 경로와 원인을 함께 반환(또는 경고 콜백/구조체)하여 `guard status`와 런처 배너에 명확히 노출 |
| **높음** | `internal/adapters/launcher/launcher.go` (L66-L110) | `EnsureAgentSkill`, `EnsureWorkspaceHook`, `sessionMgr.Create`의 에러를 `_`로 무시. 세션 생성 실패 시 `guard top` 미노출 및 훅 기록 누락 원인 추적 불가 | 실패 시 `stderr`에 경고 출력(최소 운영 관측성 확보) |
| **중간** | `internal/adapters/hook/handler.go` (L47-L90) | stdin JSON 파싱 실패 및 `AppendEvent` 실패를 무음 처리 | Fail-open 동작은 유지하되 `GUARD_DEBUG=1` 또는 `stderr` 진단 로그 경로 제공 |
| **중간** | `internal/adapters/tui/model.go` (L71-L73) | 0.5초 틱마다 `CleanStaleSessions()` 내부에서 `List()`를 호출하고 직후 `List()`를 **한 번 더 호출** (디스크 스캔·JSON 파싱 2배) | GC와 목록 조회를 단일 스캔으로 통합(예: `CleanAndList()`) |
| **낮음** | `internal/core/matcher.go` (L39) | `**` 패턴 매칭마다 `regexp.Compile`을 매번 재실행 | 패턴별 컴파일 정규식 캐싱(`sync.Map`) 도입 |

---

## 6. [카테고리 4] 코드 간결성 · 유지보수성 점검 결과

### 6.1 아키텍처 강점 (유지)
- 패키지 의존 방향(`core` ← `hub` ← `adapters` ← `cmd`)이 단방향으로 명확히 분리되어 있음.
- `internal/core`는 파일시스템 및 UI 의존성이 없는 순수 도메인 패키지로 유지됨.

### 6.2 리팩토링 대상 항목

| 심각도 | 항목 | 대상 파일 | 현황 및 문제점 | 리팩토링 목표 |
| :---: | :--- | :--- | :--- | :--- |
| **높음** | **모드 상수·정규화 SSOT 통합** | `internal/core/evaluator.go`, `internal/hub/session/manager.go`, `internal/adapters/launcher/{launcher,banner}.go`, `internal/adapters/tui/styles.go`, `cmd/guard/main.go` | `"DEV"`, `"TEST"`, `"PLAN"`, `"ALL"`, `"QA"` 문자열과 `strings.ToUpper(strings.TrimSpace(...))`가 5개 패키지에 중복 분산. `QA` 별칭도 일부 파일에만 존재 | `internal/core`에 `ModeDev`, `ModeTest`, `ModePlan`, `ModeAll` 상수 및 `NormalizeMode()` 단일 정의 |
| **중간** | **공통 유틸 중복 제거** | `cmd/guard/main.go`, `internal/adapters/launcher/banner.go`, `internal/adapters/tui/view.go`, `internal/adapters/hook/handler.go` | ① `shortSessionID`(앞 8자리 축약) 3중 중복<br/>② `AGENT_MODE`/`AGENT_ROLE` 및 `.agents/current_mode` 조회 로직이 `main.go`와 `handler.go`에 2중 중복 | ① `session.ShortID(id)`로 통합<br/>② 워크스페이스 모드 해석 함수를 단일 공용 함수로 통합 |
| **중간** | **`command.go` 중복 토크나이징·중복 블록 정리** | `internal/core/command.go` (L48-L96, L188-L268) | 동일 세그먼트에 대해 `tokenizeSegment`를 최대 3회 반복 호출. 인라인 스크립트 검사(`inspectReadSegment`/`inspectWriteSegment`) 블록이 거의 동일하게 중복 | `evaluateCommandRules`에서 세그먼트당 1회만 토크나이징하여 전달. 인라인 스크립트 검사를 공통 헬퍼로 추출 |
| **중간** | **문서·테스트 드리프트 해소** | `doc/03_tui_control_tower.md`, `internal/adapters/hook/handler_test.go`, `internal/hub/session/session_test.go` | 제거된 `상태(Active PID / Unbound)` 컬럼이 설계 문서에 잔존. 기존 테스트 2건이 삭제된 `unbound(PID=0)` 보존 스펙을 검증 중 | 설계 문서를 현재 5컬럼 구조에 맞게 현행화하고, 테스트 기대값을 신규 정책(`unbound` 비추적/즉시 GC)에 맞게 동기화 |
| **낮음** | **`main.go` 정적 문자열 분리** | `cmd/guard/main.go` (L248-L327) | `printDoc()` 80줄 마크다운이 진입점에 인라인되어 가독성 저하 | `cmd/guard/doc.go` 또는 `//go:embed` 텍스트 파일로 분리 |

### 6.3 코드 구조 점검 (DTO · 인터페이스 · Struct)

- **인터페이스 원칙 (현재 유지)**: 커스텀 인터페이스 0개, 표준 인터페이스(`io.Reader`, `io.Writer`, `tea.Model`)만 사용하는 현재 구조는 단일 파일 저장소 기반 CLI 규모에 적합하며 과잉 추상화가 없음. 향후 저장소 구현체가 2개 이상 필요해질 때만 소비자 측 소형 인터페이스를 도입.
- **DTO 경계 (현재 유지)**: `hook.Input`/`Output`은 외부 JSON 변형을 어댑터에서 깔끔하게 흡수하고 있으며, `session.Session`/`Event` 및 `core.Policy`/`Rule`은 별도 영속화 DTO를 복제하지 않는 현 구조가 실용적.

| 심각도 | 항목 | 대상 파일 | 현황 및 문제점 | 리팩토링 목표 |
| :---: | :--- | :--- | :--- | :--- |
| **중간** | **`core.EvalRequest` 구조체 도입** | `internal/core/evaluator.go` (L32) | `Evaluate(mode, policy, toolName, targetPath, cmdLine)`에 문자열 인자 4개가 나열되어 순서 혼동 및 확장 시 시그니처 변경 부담 | `EvalRequest{Mode, ToolName, TargetPath, CommandLine}` 구조체로 통합 (호환 래퍼 유지 가능) |
| **중간** | **`policy.LoadResult` 구조체 도입** | `internal/hub/policy/loader.go` (L30) | `(*Policy, string)` 다중 반환 구조로 YAML 파싱 경고 등을 함께 전달하기 어려움 | `LoadResult{Policy, Source, Warnings}` 구조체 반환으로 확장 |
| **중간** | **`cmd/guard` 진입점 `app` 구조체화** | `cmd/guard/main.go` (L18-L196) | 패키지 레벨 함수들이 `sessMgr`, `workspaceRoot`를 반복 전달하고 `os.Stdout`/`os.Exit` 직접 호출 | `app{sessMgr, workspaceRoot, stdout, stderr}` 구조체와 `run(args) int` 메서드로 분리해 간결성 및 CLI 테스트 가능성 확보 |
| **낮음** | **`tui.Model` 토스트 상태 그룹화** | `internal/adapters/tui/model.go` (L25-L27) | `toast`, `toastIsErr`, `toastUntil`이 모델 최상위에 평탄하게 혼재 | 소형 `toastState` 구조체로 묶어 관심사 분리 |

---

## 7. 리팩토링 실행 순서 (Bottom-Up + Zero Rework)

동일 파일의 중복 수정을 방지하고 회귀를 차단하기 위해 **안전망 확보 → 의존성 최하단(Bottom-Up) → 상위 어댑터 → 진입점** 순으로 진행합니다.

```text
[Step 0] 테스트 GREEN 확보 (안전망)
   ↓
[Step 1] internal/core                     (의존성 0 — 타입·보안 코어 확정)
   ↓
[Step 2] internal/hub/{policy,session}     (저장소·로더 API 계약 확정)
   ↓
[Step 3] internal/adapters/{hook,launcher,tui} (확정된 API 기반 파일당 1회 수정)
   ↓
[Step 4] cmd/guard                         (최상위 진입점 조립·분리)
   ↓
[Step 5] 문서 현행화 · -race/vet 최종 검증
```

### Step 0 — 사전 안전망 확보 (완료)
- [x] `internal/adapters/hook/handler_test.go`, `internal/hub/session/session_test.go` 기대값을 신규 `unbound` 비추적/즉시 GC 정책에 맞게 동기화하여 전체 테스트 GREEN 확보

### Step 1 — `internal/core` (순수 구조 리팩토링 완료)
- [x] `evaluator.go`: 모드 상수(`ModeDev`/`ModeTest`/`ModePlan`/`ModeAll`/`ModeQA`), `NormalizeMode()`, `IsBypassMode()`, `EvalRequest` 구조체 및 `EvaluateRequest` 추가 (기존 `Evaluate`는 호환 래퍼 유지)
- [x] `matcher.go`: 정규식 캐싱(`sync.Map`, `getCompiledGlob`) 적용
- [x] `command.go`: 세그먼트당 1회 토크나이징(`isSafeSegment`, `hasOutputRedirection`) 및 인라인 스크립트 공통 헬퍼(`inspectInlineScript`) 추출

### Step 2 — `internal/hub/policy`, `internal/hub/session` (완료)
- [x] `policy`: `LoadResult{Policy, Source, Warnings}`, `LoadDetailed*` 도입(YAML 파싱 실패 경고 수집) 및 `ResolveWorkspaceMode` 단일화
- [x] `session`: `core.NormalizeMode` 적용, `ShortID()` 추가, `sanitizeID` 선행 `.` 처리, 세션별 `syscall.Flock` 도입(Lost Update 방지), `CleanAndList()` 단일 스캔 추가

### Step 3 — `internal/adapters/{hook,launcher,tui}` (완료)
- [x] `hook`: `core.EvalRequest`, `policy.LoadDetailedWithOptions`, `policy.ResolveWorkspaceMode` 적용, `GUARD_DEBUG=1` 진단 로깅 추가
- [x] `launcher`: `policy.LoadDetailed` 파싱 경고 배너 출력, `Ensure*`/`sessionMgr.Create` 에러 `stderr` 경고, `setTerminalTitle` TTY 체크 가드, `session.ShortID`·`core.NormalizeMode` 적용
- [x] `tui`: `CleanAndList()` 단일 스캔 적용, `session.ShortID`·`core.NormalizeMode` 적용, `toastState` 소형 구조체 정리

### Step 4 — `cmd/guard` (완료)
- [x] `cliApp{sessMgr, workspaceRoot, stdin, stdout, stderr, stdinPiped}` 구조체 및 `run(args) int` 메서드 분리
- [x] `printDoc()` 별도 파일(`cmd/guard/doc.go`) 분리
- [x] `guard status`에서 `policy.LoadDetailed` 경고 출력, `CleanAndList()` 단일 스캔, `policy.ResolveWorkspaceMode`/`session.ShortID` 적용

### Step 5 — 문서 현행화 및 최종 검증
- [x] `doc/03_tui_control_tower.md` 활성 세션 테이블 명세 및 `CleanAndList` GC 현행화
- [ ] QA 최종 회귀 검증 (`go test -race ./...`)

### 후속 분리 과제 — 비즈니스 로직(보안 규칙) 확장 *(순수 리팩토링과 분리)*
- [ ] `internal/core/matcher.go`: `normalizePath`에 `path.Clean` 적용 (`..` 경로 순회 차단)
- [ ] `internal/core/command.go`: 읽기/쓰기 우회 도구 보강(`grep`/`awk`/`sed`/`cp`/`mv`/`rm` 등) 및 `$(...)`·백틱 명령 치환 내부 검사 추가
- [ ] `internal/adapters/hook` / 내장 정책: `.agents/current_mode`·`.agents/hooks.json` 쓰기 보호 규칙 검토
