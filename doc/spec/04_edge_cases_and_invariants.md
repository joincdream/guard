# [04_edge_cases_and_invariants.md] QA 블랙박스 경계 조건 · 불변 규칙 · 테스트 격리 가이드

> **대상 독자**: QA (블랙박스 테스트 설계 및 회귀 검증 기준)

---

## 1. 테스트 환경 격리 필수 규칙

`guard` 자체를 `guard test agy` (또는 `guard dev agy`) 안에서 실행하며 `go test ./...`를 돌릴 경우, 외부 런처의 환경변수가 테스트 프로세스에 상속됩니다.

- **`hook` / `policy` / `cli` 테스트 작성 시 권장 격리**:
  ```go
  t.Setenv("GUARD_SESSION_ID", "")
  t.Setenv("AGENT_MODE", "")
  t.Setenv("AGENT_ROLE", "")
  t.Setenv("GUARD_POLICY_PATH", "")
  t.Setenv("GUARD_DEBUG", "")
  ```
- **i18n 테스트 작성 시**:
  - `GUARD_LANG` 또는 `i18n.SetLang(...)`를 변경한 경우 `defer i18n.SetLang(prev)`로 원복.

---

## 2. 핵심 경계 조건 테스트 매트릭스

| 영역 | 시나리오 | 기대 결과 |
| :--- | :--- | :--- |
| **Core 평가** | `Mode`가 `" all "`, `"qa"`, `"ALL"` | 모든 `read_file`/`write_file`/`run_command` → `Allowed: true` |
| **Core 평가** | `policy == nil` 또는 `Rules` 비어 있음 | `Allowed: true` |
| **Core 명령** | `git status && cat pkg/foo_test.go` (`DEV`) | 앞 세그먼트는 안전하지만 뒤 세그먼트가 위반 → `Allowed: false` |
| **Core 명령** | `git log -p pkg/foo_test.go` (`DEV`) | `-p` 포함 시 안전 화이트리스트 제외 → `Allowed: false` |
| **Core 명령** | `go build ./... > src/internal/a.go` (`PLAN`) | 출력 리다이렉션 포함 시 안전 화이트리스트 제외 → `Allowed: false` |
| **Policy 로더** | 상위 우선순위 `guard-policy.yaml`이 문법 오류 YAML이고 하위/builtin이 유효함 | 하위 유효 정책 또는 `builtin` 로드 + `LoadResult.Warnings`에 오류 파일 경로·원인 1건 이상 포함 |
| **Policy 로더** | 후보 경로 파일이 아예 존재하지 않음 | `Warnings` 비어 있음 (정상 폴백) |
| **Session GC** | `alive(현재 PID)`, `dead(종료 PID)`, `unbound(PID: 0)` 혼재 후 `CleanAndList()` | `alive`만 생존 반환, `dead`와 `unbound` 모두 삭제 목록에 포함 |
| **Session 동시성** | 동일 세션에 10개 고루틴이 `AppendEvent(deny)` 동시 호출 (`go test -race`) | 데이터 레이스 0건, 최종 `ViolationCount == 10`, 이벤트 10건 모두 보존 |
| **Session ID** | `ID: ".hidden"` 또는 `"../evil"` 생성 후 `List()` | 디렉토리 탈출 없음, 숨김 파일 필터에 걸리지 않고 `List()`에 정상 노출 |
| **Hook** | 빈 stdin 또는 깨진 JSON `{invalid` | 에러 없이 `{"decision":"allow"}` 반환 (Fail-open) |
| **Hook** | 추적되지 않은 `sessionId`로 `Handle` 호출 | 평가는 정상 수행하되 세션 디렉토리에 `PID: 0` 파일을 **생성하지 않음** |
| **Hook** | `GUARD_SESSION_ID`에 존재하지 않는 ID가 설정되어 있고, 페이로드 `sessionId`가 `sessionMgr`에 존재 | 페이로드 `sessionId`의 세션 모드가 적용되고 해당 세션에 이벤트 기록 |
| **Launcher** | `bytes.Buffer`(비-TTY)를 `stdout`으로 전달해 `Run` 실행 | 출력에 ANSI OSC 타이틀 시퀀스(`\033]0;`)가 섞이지 않음 |
| **Launcher** | 기존 `.agents/hooks.json`에 `"obsidian-chatlog"` 존재 시 `EnsureWorkspaceHook` 실행 | `"obsidian-chatlog"` 키 100% 보존 + `"guard"` 키 병합 |
| **TUI** | 좁은 터미널(예: width=80)에서 한글/영문(`ko`/`en`) 모드 전환 후 `View()` 렌더링 | 활성 세션 행이 2줄로 줄바꿈되지 않고 `PID`/`Active` 문자열 미포함 |

---

## 3. 현재 미구현(후속 과제) 항목 — 현 시점 테스트 대상 제외

아래 항목은 [07_refactoring_plan.md](file:///home/yundream/mytask/kms/guard/doc/07_refactoring_plan.md)의 **후속 보안 규칙 확장 과제**로 분리되어 있으므로, 현재 리팩토링 회귀 검증 시 실패 케이스로 기대하지 않습니다.

1. `matcher.go`의 `..` 경로 순회 정규화 (`path.Clean`)
2. `command.go`의 확장 우회 도구(`grep`, `rg`, `awk`, `sed` 조회, `cp`, `mv`, `rm`, `patch`, `dd`, `truncate`) 및 `$(...)`·백틱 명령 치환 내부 검사
3. `.agents/current_mode`, `.agents/hooks.json` 쓰기 보호 규칙
