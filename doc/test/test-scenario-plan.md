# QA 단계적 블랙박스 테스트 시나리오 계획서

- **작성일**: 2026-11-20
- **작성자**: QA
- **기준 문서**: [doc/spec/](../spec/) (`01_core_policy_contract.md` ~ `04_edge_cases_and_invariants.md`)
- **대상 프로젝트**: `guard` (Antigravity CLI Guardrail)

---

## 0. QA 테스트 원칙 및 설계 기준

### 0.1 핵심 원칙
1. **결함 발견 최우선 (Defect-Hunting First)**
   - 테스트를 통과시키는 것이 목적이 아니라, **계약 위반·보안 우회·경계값 오류·동시성 레이스·상태 오염**을 최대한 찾아내는 것이 목적이다.
2. **전수 커버리지 (Happy + Edge + Adversarial)**
   - 모든 공개 계약의 정상 경로(Happy Path)뿐만 아니라 경계값(Boundary), 잘못된 입력(Negative), 보안 우회 시도(Adversarial), 동시성 충돌(Concurrency)을 빠짐없이 포함한다.
3. **가짜 성공 금지 (No Fake-Green Tests)**
   - 통과를 맞추기 위한 하드코딩, 임의의 통과용 값 세팅, 핵심 필드를 검증하지 않는 느슨한 Assertion(`err == nil`만 체크 등)을 엄격히 금지한다.
   - 모든 테스트는 외부 환경변수(`GUARD_SESSION_ID`, `AGENT_MODE`, `AGENT_ROLE`, `GUARD_POLICY_PATH`, `GUARD_DEBUG`, `GUARD_LANG`)를 명시적으로 격리(`t.Setenv`)하여 우연한 통과/실패를 원천 차단한다.

### 0.2 현 단계 테스트 제외 범위 (미구현 후속 과제)
[04_edge_cases_and_invariants.md](../spec/04_edge_cases_and_invariants.md) §3에 따라 아래 항목은 현 단계 실패 판정 대상에서 제외하고 별도 추적한다.
- `matcher.go`의 `..` 경로 순회 정규화(`path.Clean`)
- `command.go`의 확장 우회 도구(`grep`, `rg`, `awk`, `sed` 조회, `cp`, `mv`, `rm`, `patch`, `dd`, `truncate`) 및 `$(...)`/백틱 명령 치환 내부 검사
- `.agents/current_mode`, `.agents/hooks.json` 쓰기 보호 규칙

---

## 1. 단계적 실행 로드맵 (Bottom-Up & Risk-First)

| 단계 | 대상 패키지 | 우선순위 | 핵심 검증 목표 |
| :--- | :--- | :---: | :--- |
| **Phase 1** | `guard/internal/core` | **P0** | 정책 평가 무결성, 패턴 매칭 오탐/미탐, 셸 보안 우회 차단 |
| **Phase 2** | `guard/internal/hub/policy`<br>`guard/internal/hub/session` | **P0** | 정책 폴백·경고 정확성, 세션 원자성·ID 탈출 방어·GC·Flock 동시성(`-race`) |
| **Phase 3** | `guard/internal/adapters/hook`<br>`guard/internal/adapters/launcher` | **P1** | Hook 페이로드·Fail-open·세션 격리, Launcher PID 추적·비TTY 오염 방지·병합 보존 |
| **Phase 4** | `guard/internal/i18n`<br>`guard/internal/adapters/tui` | **P2** | 다국어 폴백·카탈로그 정합성, TUI 키 상태 전이·좁은 화면 렌더링 불변성 |
| **Phase 5** | `guard/cmd/guard` (CLI E2E) | **P2** | 서브커맨드 종료 코드, 파이프/TTY 분기, 워크스페이스·세션 격리 통합 검증 |

---

## Phase 1: Core 정책 평가 엔진 & 패턴 매칭 (`guard/internal/core`)

### 1.1 모드 정규화 및 Bypass 경계
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P1-001` | 정상 | 모드 정규화 기본 동작 | `"dev"`, `" TEST "`, `"\tPlan\n"`, `""`, `"   "` | 각각 `"DEV"`, `"TEST"`, `"PLAN"`, `""`, `""` 반환 | 공백/탭/개행 미제거, 대소문자 불일치 |
| `TC-P1-002` | 정상 | Bypass 모드 판정 | `IsBypassMode("ALL")`, `" all "`, `"qa"`, `"QA"`, `"DEV"`, `"TEST"`, `"PLAN"`, `""` | `ALL`/`QA` 변형만 `true`, 나머지 모두 `false` | `QA` 누락, 소문자/공백 미처리, 빈 문자열 오탐 |
| `TC-P1-003` | 경계 | `nil` 정책 및 빈 규칙 목록 | `policy == nil` 또는 `Policy{Rules: nil}` / `Rules: []Rule{}` | `Decision{Allowed: true, RuleName: "", Reason: ""}` | nil 역참조 패닉 |
| `TC-P1-004` | 정상 | Bypass 모드에서 차단 대상 접근 | `Mode: " all "` / `"qa"`, `read_file`로 `*_test.go` 또는 `write_file`로 보호 경로 | `Allowed: true` (규칙 매칭 무시) | Bypass 검사가 규칙 평가 뒤에 위치하는 버그 |
| `TC-P1-005` | 경계 | `Evaluate` 래퍼 동등성 | 동일 입력으로 `Evaluate` vs `EvaluateRequest` 호출 (허용/차단 각 케이스) | 반환 `Decision` 구조체 100% 동일 | 래퍼 인자 순서 뒤바뀜 |

### 1.2 패턴 매칭 (`MatchPattern`) & 규칙 평가 순서
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P1-006` | 정상 | 접미사/확장자 매칭 | `MatchPattern("pkg/a_test.go", "*_test.go")`, `MatchPattern("pkg/a.go", "*_test.go")` | 각각 `true`, `false` | 접미사 오탐 |
| `TC-P1-007` | 경계 | 디렉토리 경계 오탐 방지 | `MatchPattern("src/internal_extra/a.go", "src/internal/**")`, `MatchPattern("mytests/a.go", "**/tests/**")` | 모두 `false` | 단순 `strings.HasPrefix`/`Contains` 기반 오탐 |
| `TC-P1-008` | 경계 | 다중 디렉토리 `**` 루트/중첩/절대경로 | `tests/a.go`, `a/b/tests/c.go`, `/abs/path/tests/c.go`, `./tests/a.go` vs `**/tests/**` | 모두 `true` | `./` 미제거, 루트 레벨 `**/` 미매칭, 절대경로 실패 |
| `TC-P1-009` | 경계 | `ExcludePatterns` 우선 면제 | `Patterns: ["**/*"]`, `ExcludePatterns: ["*_test.go", "doc/**"]`에서 `doc/spec.md`, `a_test.go`, `internal/a.go` | `doc/spec.md`·`a_test.go`는 `Allowed: true`, `internal/a.go`는 `Allowed: false` | Exclude 조건 누락 또는 일부 패턴만 검사 |
| `TC-P1-010` | 경계 | 규칙 선언 순서 (최초 매칭 우선) | 동일 경로에 매칭되는 Rule A(1번), Rule B(2번) 선언 | `Allowed: false`, `RuleName == "Rule A"`, `Reason == RuleA.Message` | 후순위 규칙이 선순위 규칙을 덮어쓰는 버그 |
| `TC-P1-011` | 경계 | 글로벌 규칙 (`Rule.Mode == ""`) | `Rule.Mode: ""` 규칙이 `DEV`, `TEST`, `PLAN`에서 동작하고 `ALL`/`QA`에서는 우회되는지 | `DEV`/`TEST`/`PLAN` → `false`, `ALL`/`QA` → `true` | 빈 모드 규칙 무시 또는 Bypass 무시 |
| `TC-P1-012` | 예외 | 빈 경로/빈 패턴 입력 | `MatchPattern("", "*.go")`, `MatchPattern("a.go", "")`, `MatchPattern("", "")` | 패닉 없이 `false` 반환 | 빈 문자열 슬라이싱 패닉 |

### 1.3 파일 도구 (`read_file`, `write_file`, `edit_file`) 평가
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P1-013` | 정상 | 도구-타입 매핑 정확성 | `read` 규칙에 `read_file` vs `write_file`/`edit_file`; `write` 규칙에 `write_file`/`edit_file` vs `read_file` | `read_file`은 `read` 규칙만, `write_file`/`edit_file`은 `write` 규칙만 평가 | `edit_file`이 `write` 규칙에서 누락되는 버그 |
| `TC-P1-014` | 예외 | 미지원/알 수 없는 `ToolName` | `ToolName: "unknown_tool"`, `TargetPath: "a_test.go"`, `CommandLine: ""` | `Allowed: true` (패닉 없음) | 알 수 없는 도구 오차단 |

### 1.4 셸 명령어 파싱 & 보안 우회 공격 (Adversarial)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P1-015` | 공격 | 화이트리스트 앞세우기 + 제어 연산자 우회 | `DEV`: `git status && cat a_test.go`, `go test ./... ; head a_test.go`, `go vet ./... \|\| tail a_test.go`, `git branch \| cat a_test.go`, `git status\ncat a_test.go` | 모든 변형 `Allowed: false` | 첫 세그먼트만 화이트리스트 검사 후 조기 `Allow` 반환하는 버그 |
| `TC-P1-016` | 공격 | 화이트리스트 명령 + 출력 리다이렉션 | `PLAN`: `git status > src/internal/a.go`, `go build ./... >> src/cmd/a.go`, `go vet ./... 2> src/internal/b.go`, `git branch &> src/internal/c.go` | 모든 변형 `Allowed: false` | 리다이렉션 존재 시 화이트리스트 면제 미해제 |
| `TC-P1-017` | 정상 | 안전한 화이트리스트 명령 허용 | `DEV`/`PLAN`: `git status`, `git branch`, `git commit -m "msg"`, `git log --oneline -n 5`, `go build ./...`, `go vet ./...` | 모두 `Allowed: true` | 과도한 차단(False Positive) |
| `TC-P1-018` | 공격 | Git 내용 조회 우회 | `DEV`: `git show HEAD:pkg/a_test.go`, `git diff pkg/a_test.go`, `git checkout -- pkg/a_test.go`, `git log -p pkg/a_test.go`, `git log --patch pkg/a_test.go`, `git log -u pkg/a_test.go` | 모두 `Allowed: false` | `rev:path` 파싱 누락, `git log -p/--patch/-u` 화이트리스트 오허용 |
| `TC-P1-019` | 공격 | 파일 뷰어 전종 + 플래그/와일드카드 | `DEV`: `cat`, `head -n 20`, `tail -f`, `more`, `less`, `strings`, `nl`, `tac`, `bat` + `pkg/a_test.go` 및 `pkg/*_test.go` | 모두 `Allowed: false` | 일부 뷰어 누락, 플래그 인자 뒤 경로 미추출 |
| `TC-P1-020` | 공격 | 입력 리다이렉션 변형 | `DEV`: `wc -l < pkg/a_test.go`, `tr a A <pkg/a_test.go` (공백 없음) | 모두 `Allowed: false` | `<` 뒤 공백 없는 경로 미파싱 |
| `TC-P1-021` | 공격 | 쓰기 도구(`tee`, `sed -i`) 변형 | `PLAN`: `echo x \| tee src/internal/a.go`, `echo x \| tee -a src/internal/a.go`, `sed -i 's/a/b/' src/internal/a.go`, `sed --in-place 's/a/b/' src/internal/a.go` | 모두 `Allowed: false` | `tee -a` 플래그 건너뛰기 실패, `sed --in-place` 누락 |
| `TC-P1-022` | 공격 | 인라인 스크립트 읽기·쓰기 우회 | `DEV`/`PLAN`: `python3 -c "open('pkg/a_test.go').read()"`, `node -e "fs.readFileSync('pkg/a_test.go')"`, `ruby -e ...`, `perl -e ...`, `bash -c "cat pkg/a_test.go"`, `sh -c ...`, `zsh -c ...`, `node --eval ...` | 대상 경로가 차단 규칙에 매칭되면 모두 `Allowed: false` | 인터프리터 일부 누락, `--eval` 미지원, 따옴표 내 경로 미추출 |
| `TC-P1-023` | 정상 | 인라인 스크립트 무해한 실행 허용 | `DEV`: `python3 -c "print(1+1)"`, `node -e "console.log('ok')"` | `Allowed: true` | 인라인 스크립트 무조건 차단(False Positive) |
| `TC-P1-024` | 경계 | 따옴표 내부 연산자 보존 | `DEV`: `git commit -m "fix a && b ; c \| d"` | `Allowed: true` (따옴표 내부를 세그먼트로 잘못 쪼개지 않음) | 단순 문자열 `Split`으로 인한 파싱 오류 |
| `TC-P1-025` | 경계 | `TEST` 모드에서 `ExcludePatterns` 셸 허용 | `TEST`: `cat pkg/a_test.go`, `cat doc/spec/01.md` vs `cat internal/core/eval.go` | 앞 2건 `Allowed: true`, 프로덕션 파일 `Allowed: false` | 셸 검사에서 `ExcludePatterns` 미적용 |

---

## Phase 2: Policy 로더 & Session Hub (`guard/internal/hub/policy`, `guard/internal/hub/session`)

### 2.1 Policy 로더 우선순위 · 폴백 · 경고 (`guard/internal/hub/policy`)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P2-001` | 정상 | 5단계 우선순위 정합성 | `EnvPath`, `CWD`, `WorkspaceRoot/.agents`, `HomeDir/.config/guard` 모두 유효 YAML 배치 후 상위부터 순차 제거 | 1→2→3→4→`builtin` 순서로 정확히 로드, `Source` 경로 일치, `Warnings` 0건 | 우선순위 역전 또는 후보 경로 오타 |
| `TC-P2-002` | 예외 | 상위 YAML 문법 오류 → 하위 폴백 + 경고 | `EnvPath`에 깨진 YAML, `CWD`에 유효 YAML | `CWD` 정책 로드, `len(Warnings) == 1`, 경고에 오류 파일 경로 포함 | 파싱 실패 시 무경고 무시 또는 전체 중단 |
| `TC-P2-003` | 예외 | 상위 YAML이 유효하나 `rules: []` (0건) | `EnvPath`에 `version: "1"\nrules: []`, `CWD`에 유효 정책 | `CWD` 정책 로드 + `Warnings`에 경고 기록 | 빈 정책을 유효한 것으로 간주해 모든 가드레일이 해제되는 치명적 버그 |
| `TC-P2-004` | 예외 | 모든 후보가 손상된 YAML | 1~4순위 파일 모두 손상 YAML | `DefaultPolicy()` 반환, `Source == "builtin"`, `len(Warnings) == 4` | 다중 경고 누락 또는 폴백 실패 |
| `TC-P2-005` | 경계 | 후보 파일이 전혀 없는 경우 | 빈 임시 디렉토리들 전달 | `Source == "builtin"`, `len(Warnings) == 0` | 미존재 파일을 경고로 오인 |
| `TC-P2-006` | 예외 | `LoadFile`에 디렉토리 경로 / 미존재 파일 | `LoadFile(t.TempDir())`, `LoadFile("/nonexistent/p.yaml")` | non-nil error 반환 | 에러 무시 |
| `TC-P2-007` | 정상 | `ResolveWorkspaceMode` 4단계 폴백 | `AGENT_MODE` → `AGENT_ROLE` → `.agents/current_mode` (`" test \n"`) → 기본값 `DEV` | 우선순위대로 정규화된 대문자 모드 반환; 빈 `.agents/current_mode`는 `DEV` 폴백 | 공백/개행 미처리, 우선순위 역전 |

### 2.2 Session Hub 영속화 · 보안 · GC · 동시성 (`guard/internal/hub/session`)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P2-008` | 예외 | `Create` 잘못된 입력 방어 | `Create(nil)`, `Create(&Session{ID: ""})`, `Create(&Session{ID: "   "})` | 모두 non-nil error 반환, 파일 미생성 | nil 패닉, 빈 파일명 `.json` 생성 |
| `TC-P2-009` | 정상 | `Create` 기본값 초기화 및 원자적 저장 | `Mode: ""` 또는 `"plan"`, `Events: nil`로 생성 | `Mode` 정규화(`DEV`/`PLAN`), `CreatedAt`/`UpdatedAt` non-zero, `Get()` 시 `Events != nil`, 디렉토리에 `.tmp` 잔존 0건 | 빈 모드 미정규화, `Events` nil 반환, 임시 파일 누수 |
| `TC-P2-010` | 공격 | Path Traversal & 숨김 파일 ID 방어 | `ID: "../evil"`, `"a/b"`, `"a\\b"`, `".hidden"`, `"..hidden"` 생성 후 `SessionPath`, `Get`, `List` | 파일이 반드시 `mgr.Dir()` 내부에 저장(디렉토리 탈출 0건), `.` 선행 ID도 `_` 치환되어 `List()`에서 누락되지 않음 | 경로 탈출 취약점, 숨김 파일 필터에 의한 정상 세션 증발 |
| `TC-P2-011` | 경계 | `List()` 정렬 불변성 및 손상 파일 내성 | `UpdatedAt`이 다른 세션 3개 + 동일 `UpdatedAt`에 ID `"b"`, `"a"` + 깨진 `broken.json` + `.lock` 파일 | 에러 없이 유효 세션만 반환; `UpdatedAt` 내림차순, 동률 시 `"a"` → `"b"` 오름차순 | 정렬 불안정, 깨진 JSON 1건으로 전체 `List()` 실패 |
| `TC-P2-012` | 경계 | `Delete` 멱등성 & `DeleteAll` 카운트 정확성 | 존재하지 않는 ID `Delete`, 존재하는 ID `Delete`(연관 `.lock` 포함), `DeleteAll` (`.json` 2개 + `.lock` 2개) | 미존재 삭제 `err == nil`, 삭제 후 `.json` 및 `.lock` 모두 제거, `DeleteAll` 반환값 `2` (`.lock` 미카운트) | `.lock` 파일 고아 잔존, `DeleteAll`이 `.lock`까지 카운트 |
| `TC-P2-013` | 경계 | `AppendEvent` 로테이션 경계값 (50 / 51 / 100건) | 50건 추가 → 1건 추가(51건) → 49건 추가(100건), `allow`/`deny` 교차 | 50건 유지, 가장 오래된 이벤트부터 탈락, `ViolationCount`는 로테이션과 무관하게 누적 총 `deny` 건수 유지 | off-by-one(49 또는 51개 유지), 로테이션 시 `ViolationCount` 감소 버그 |
| `TC-P2-014` | 정상 | `CleanAndList` / `CleanStaleSessions` GC 정책 | `alive(os.Getpid())`, `dead(종료 PID)`, `unbound0(PID: 0)`, `unboundNeg(PID: -5)` 혼재 | `alive` 1건만 생존 반환; `dead`, `unbound0`, `unboundNeg` 3건 모두 삭제 목록 포함 및 디스크에서 삭제 | `PID <= 0` 미삭제(유령 세션 잔존), `IsProcessAlive` 오판 |
| `TC-P2-015` | 동시성 | `Flock` 동시 쓰기 무결성 (`go test -race`) | 20개 고루틴이 동일 세션에 `AppendEvent(deny)` 5회씩(총 100회) + 5개 고루틴이 `UpdateMode` 동시 실행 | `-race` 레이스 0건, 에러 0건, 최종 JSON 파싱 정상, `ViolationCount == 100`, `len(Events) == 50` | 파일 락 누락으로 인한 Lost Update 및 JSON 부분 쓰기 손상 |
| `TC-P2-016` | 경계 | `ShortID` 경계값 | 길이 0, 7, 8, 9, 36자 문자열 | 8자 이하 원문 반환, 9자 이상 앞 8자리 반환 | off-by-one 슬라이싱 |

---

## Phase 3: 어댑터 계층 — Hook & Launcher (`guard/internal/adapters/hook`, `guard/internal/adapters/launcher`)

### 3.1 Hook Adapter (`guard/internal/adapters/hook`)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P3-001` | 예외 | Fail-open 계약 (빈 입력 / 깨진 JSON) | `""`, `"   "`, `"{invalid"`, `"null"` | 에러 없이 `{"decision":"allow"}` 출력, `stderr` 무출력 | 잘못된 JSON으로 도구 실행 전체가 마비되는 Fail-closed 버그 |
| `TC-P3-002` | 정상 | 다중 입력 JSON 스키마 호환성 | ① `toolCall.{name,args}` ② `tool_name`/`tool_args` ③ 최상위 `Path`/`path`/`CommandLine`/`command_line` ④ `sessionId`/`session_id`/`conversationId` | 모든 변형에서 동일하게 `DEV` 차단(`deny`) 및 이벤트 `Target` 정확 기록 | 특정 클라이언트 JSON 포맷 미인식으로 인한 정책 우회 |
| `TC-P3-003` | 경계 | 세션 해석 우선순위 & 미존재 `GUARD_SESSION_ID` 우회 (DEF-001 회귀) | `GUARD_SESSION_ID="nonexistent-outer"`, `AGENT_MODE="TEST"`, 페이로드 `sessionId="real-dev-sess"` (`sessionMgr`에 `DEV`로 존재) | 외부 환경변수에 가려지지 않고 `real-dev-sess`(`DEV`)가 매칭되어 `*_test.go` 접근 `deny` + 해당 세션에 이벤트 기록 | 외부 런처 환경변수가 내부 추적 세션을 덮어쓰는 회귀 버그 |
| `TC-P3-004` | 경계 | 미등록 세션 폴백 & `PID: 0` 비생성 계약 | `sessionMgr`에 없는 `sessionId: "unknown-1"`, `AGENT_MODE="DEV"` | 정책 평가는 `DEV`로 정상 수행(`deny`), 단 `sessionMgr.List()`는 여전히 **0건** (`PID: 0` 파일 미생성) | 미추적 세션 자동 생성으로 인한 유령 세션 재발 |
| `TC-P3-005` | 정상 | 차단 시 `stdout`/`stderr` 출력 계약 | `deny` 발생 케이스 (`GUARD_DEBUG=""`) | `stdout` 유효 JSON(`decision=="deny"`, `reason!=""`), `stderr`에 `reason` 1줄 출력 | `stderr` 누락으로 사용자에게 차단 사유 미노출 |
| `TC-P3-006` | 경계 | 진단 로그(`GUARD_DEBUG`) 토글 | `allow` 케이스에서 `GUARD_DEBUG=""` vs `GUARD_DEBUG="1"` | 미설정 시 `stderr` 완전 비어 있음; `"1"` 설정 시 `stderr`에 `[guard:debug]` 포함 | 디버그 로그 상시 노출로 stdout/stderr 오염 |

### 3.2 Launcher Adapter (`guard/internal/adapters/launcher`)
| ID          | 구분  | 시나리오                                     | 입력 조건                                                                            | 기대 결과 (Assertion)                                                                                              | 결함 탐지 포인트                           |
| :---------- | :-: | :--------------------------------------- | :------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------- | :---------------------------------- |
| `TC-P3-007` | 예외  | 빈 `cmdArgs` 실행 방어                        | `l.Run("DEV", ws, nil)` 및 `l.Run("DEV", ws, []string{})`                         | 종료 코드 `1`, `stderr` 비어 있지 않음, 세션 파일 미생성                                                                        | 인덱스 패닉                              |
| `TC-P3-008` | 정상  | 자식 프로세스 환경변수 주입 · PID 등록 · 종료 시 정리       | 자식 명령으로 `sh -c 'echo "$AGENT_MODE:$GUARD_SESSION_ID"; ls <sessDir>'` 실행          | 자식에게 `AGENT_MODE`·`GUARD_SESSION_ID` 주입 확인, 실행 중 실제 자식 PID(`>0`) 세션 파일 존재, `Run()` 종료 후 세션 파일 삭제 완료, 종료 코드 `0` | 세션 누수(미삭제), `PID: 0` 저장, 환경변수 미전달   |
| `TC-P3-009` | 예외  | 자식 프로세스 비정상 종료 코드 전파                     | `sh -c 'exit 42'` 실행                                                             | `Run()` 반환값 `42`, 세션 파일 정상 정리(`defer Delete`)                                                                  | 종료 코드 삼킴(0 또는 1로 고정), 실패 시 세션 정리 누락 |
| `TC-P3-010` | 경계  | 비-TTY(`bytes.Buffer`) ANSI OSC 미오염       | `stdout`에 `&bytes.Buffer{}` 전달 후 `Run()` 실행                                      | `stdout`에 `\033]0;` 미포함                                                                                        | 파이프/로그 출력에 터미널 제어 문자 혼입             |
| `TC-P3-011` | 정상  | 배너 정책 경고 출력                              | `PrintBanner(&buf, BannerInfo{PolicyWarnings: []string{"broken yaml"}})`         | 출력에 `⚠️  [Policy Warning]` 및 경고 내용 포함                                                                          | 정책 파싱 실패가 사용자에게 침묵 처리되는 버그          |
| `TC-P3-012` | 정상  | `EnsureAgentSkill` 최초 생성 및 기존 파일 보존      | 빈 `homeDir` 1차 호출 → 파일 내용 임의 수정 → 2차 호출                                          | 1차 `created==true`, 2차 `created==false` 및 수정 내용 100% 보존                                                        | 사용자 커스텀 스킬 덮어쓰기                     |
| `TC-P3-013` | 경계  | `EnsureWorkspaceHook` 병합 보존 & 손상 JSON 방어 | ① 빈 워크스페이스 ② 기존 `"obsidian-chatlog"` 포함 `hooks.json` ③ 깨진 `{broken` `hooks.json` | ① 생성(`updated==true`), `HasWorkspaceHook==true` ② 기존 키 100% 보존 + `"guard"` 병합 ③ 에러 반환 및 기존 깨진 파일 내용 미덮어쓰기      | 타 도구 훅 삭제, 손상 파일 강제 덮어쓰기            |

---

## Phase 4: i18n & TUI 관제탑 (`guard/internal/i18n`, `guard/internal/adapters/tui`)

### 4.1 i18n (`guard/internal/i18n`)
| ID          | 구분  | 시나리오                                        | 입력 조건                                                                                       | 기대 결과 (Assertion)                                                   | 결함 탐지 포인트                        |
| :---------- | :-: | :------------------------------------------ | :------------------------------------------------------------------------------------------ | :------------------------------------------------------------------ | :------------------------------- |
| `TC-P4-001` | 정상  | 로케일 감지 우선순위                                 | `DetectLocaleFromLookup`으로 `GUARD_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > `"en"` 순차 테스트 | 우선순위 일치, `ko_KR.UTF-8` → `"ko"`, `C`/`POSIX`/`fr_FR.UTF-8` → `"en"` | 환경변수 우선순위 오류, 미지원 로케일 폴백 실패      |
| `TC-P4-002` | 정상  | 번역 키 폴백 체인                                  | 활성 언어(`ko`) 존재 키, 미존재 키(`unknown.key`)                                                      | 존재 키는 한국어 번역, 미존재 키는 원문 `"unknown.key"` 반환                          | 미존재 키 빈 문자열 반환                   |
| `TC-P4-003` | 불변  | 카탈로그(`en.json`, `ko.json`) 키·포맷 동사 100% 정합성 | 양쪽 카탈로그 전수 비교                                                                               | 누락/잉여 키 0건, 모든 키의 `%s`/`%d` 등 포맷 동사 개수·타입 일치                        | 번역 누락 또는 `fmt.Sprintf` 런타임 포맷 깨짐 |
| `TC-P4-004` | 경계  | `RelativeTime` 시간 구간 경계                     | 현재 시각 기준 `0s`, `30s`, `5m`, `3h`, `2d` 전 (`ko`/`en` 각각)                                     | 각 구간별 비어 있지 않은 유효 상대 시간 문자열 반환, 패닉 없음                               | 음수/경계값 분기 오류                     |

### 4.2 TUI Control Tower (`guard/internal/adapters/tui`)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P4-005` | 경계 | 빈 세션 목록에서 모든 조작 키 입력 | 세션 0개 상태에서 `↑/k`, `↓/j`, `d/t/p/a`, `c`, `Space`, `?`, `r` 입력 | 패닉 0건, 안전한 빈 상태 화면 렌더링 | 빈 슬라이스 인덱스 아웃오브바운드 패닉 |
| `TC-P4-006` | 정상 | 커서 이동 및 모드 전환 / 차단 리셋 영속 반영 | 활성 세션 2개 생성 → `↓` 이동 → `t` 입력(TEST 전환) → `c` 입력(차단 리셋) | `sessionMgr.Get()`으로 실제 디스크 세션의 `Mode=="TEST"`, `ViolationCount==0` 반영 확인 | UI 메모리만 바뀌고 Session Hub에 미저장되는 버그 |
| `TC-P4-007` | 정상 | 모달 및 도움말 오버레이 토글 | `Space` → `Esc`, `?` → `Esc`, `q`/`Ctrl+C` | 모달 열림/닫힘 상태 및 `View()` 변화 확인, `q`/`Ctrl+C` 시 `tea.Quit` 명령 반환 | `Esc` 미동작으로 모달 탈출 불가 |
| `TC-P4-008` | 경계 | 좁은 터미널(`width=80`) 5컬럼 1줄 렌더링 불변성 | `ko`/`en` 각각 `tea.WindowSizeMsg{Width: 80, Height: 24}` 전달 후 긴 워크스페이스 경로 세션 렌더링 | 활성 세션 행이 2줄로 줄바꿈되지 않음, 출력에 `PID`/`Active` 헤더 미포함 | 한글 2-cell 폭 미계산(`len()` 오용)으로 테이블 깨짐 |

---

## Phase 5: CLI End-to-End 통합 테스트 (`guard/cmd/guard`)

> **사전 조건**: 실제 `~/.guard/sessions` 오염 방지를 위해 테스트 프로세스의 `HOME`을 `t.TempDir()`로 격리하고, `guard` 바이너리를 임시 경로에 1회 빌드하여 서브프로세스로 실행한다.

| ID | 구분 | 시나리오 | 실행 명령 / 입력 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-P5-001` | 정상 | 도움말 및 알 수 없는 서브커맨드 | `guard help`, `guard -h`, `guard --help` vs `guard unknown-cmd` | 도움말 종료 코드 `0`; 알 수 없는 커맨드 종료 코드 `1` + `stderr` 오류 + 도움말 | 종료 코드 `0` 오반환 |
| `TC-P5-002` | 정상 | 파이프 stdin Hook 실행 (`guard` / `guard hook`) | `AGENT_MODE=DEV`에서 `{"tool_name":"read_file","tool_args":{"Path":"a_test.go"}}` 파이프 입력 | 종료 코드 `0`, `stdout`에 `"decision":"deny"`, `stderr`에 차단 사유 출력 | 파이프 감지 실패로 도움말 출력 |
| `TC-P5-003` | 정상 | 워크스페이스 모드 전환 (`guard dev\|test\|plan\|all`) | 임시 워크스페이스에서 `guard test` 실행 | 종료 코드 `0`, `<ws>/.agents/current_mode` 내용이 `TEST`로 저장 | 파일 미생성 또는 개행/대문자 정규화 누락 |
| `TC-P5-004` | 정상 | 격리 실행 (`guard run` / `guard dev <cmd>`) | `guard run TEST sh -c 'echo $AGENT_MODE'` | 종료 코드 `0`, `stdout`에 `TEST` 포함, 종료 후 임시 `HOME/.guard/sessions`에 잔존 세션 0건 | 자식 종료 코드 미전파, 세션 미정리 |
| `TC-P5-005` | 정상 | 상태 조회 (`guard status`) + 정책 파싱 경고 노출 | 깨진 `guard-policy.yaml`이 있는 워크스페이스에서 `guard status` 실행 | 종료 코드 `0`, 워크스페이스·모드·정책 출처(`builtin`) 및 경고 메시지 출력 | 정책 오류 시 `status` 크래시 또는 경고 누락 |
| `TC-P5-006` | 정상 | 세션 정리 (`guard clean`, `guard clean --all`) & 리셋 (`guard reset <id>`) | 사망/미바인딩/활성 세션 배치 후 `clean`, `reset <alive-id>`, `clean --all` 순차 실행 | `clean` 후 활성만 잔존, `reset` 후 `ViolationCount==0`, `clean --all` 후 0건 | CLI 인자 파싱 오류, `--all` 미동작 |
| `TC-P5-007` | 정상 | 문서/샘플 출력 (`guard doc`, `guard sample`) | `guard doc`, `guard sample` | 종료 코드 `0`, 비어 있지 않은 유효 스펙/YAML 샘플 출력 | 내장 리소스 누락 |

---

## 2. 테스트 구현 전 확인 필요 사항 (Open Items)

Phase 1~4는 기존 `*_test.go` 패턴과 `doc/spec`을 조합해 즉시 구현 착수 가능하나, 아래 항목은 개발팀과 최종 확인 시 Assertion 정밀도를 더욱 높일 수 있다.
1. **`DefaultPolicy()` 내장 패턴 목록**: `DEV`/`PLAN`/`TEST` 기본 규칙의 정확한 `Patterns`·`ExcludePatterns` (예: `doc/**` vs `docs/**`, `src/internal/**` vs `internal/**`).
2. **`cmd/guard` 세션 디렉토리 격리 방식**: `HOME` 환경변수 오버라이드만으로 `session.DefaultDir()`가 완전 격리되는지 여부.
3. **`guard reset <nonexistent>`**: 존재하지 않는 세션 ID 전달 시 기대 종료 코드(`1` 확정 여부).
