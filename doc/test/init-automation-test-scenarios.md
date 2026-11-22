# 초기 설정 자동화 및 자가 진단 QA 테스트 시나리오 (`guard init` / `guard doctor`)

- **작성일**: 2026-11-22
- **작성자**: QA (`TEST` 모드)
- **핸드오프 티켓**: `hnd-20261122-e34e80bc` (`DEV(0c6f8f4b) -> TEST`)
- **기준 문서**:
  - [06_init_automation_contract.md](../spec/06_init_automation_contract.md) (필수 계약서)
  - [03_adapters_cli_contract.md](../spec/03_adapters_cli_contract.md) (Launcher · i18n · CLI 계약서)
  - [init-automation-qa-test-brief.md](./init-automation-qa-test-brief.md) (QA 테스트 가이드)

---

## 0. 테스트 원칙 및 격리 기준

### 0.1 핵심 원칙
1. **결함 발견 최우선 (Defect-Hunting First)**: 계약 위반, 비파괴 보장 깨짐(데이터 손실), 멱등성 붕괴, 잘못된 Exit Code, JSON 출력 오염을 찾아내는 데 초점을 둔다.
2. **전수 커버리지 (Happy + Boundary + Negative + Adversarial)**: 정상 경로 외에 우선순위 충돌, 손상된 파일 보호, 잘못된 플래그/인자, 심볼릭 링크 경로 불일치 등 경계·예외 케이스를 모두 포함한다.
3. **완전한 환경 격리 (No Host Contamination)**:
   - 모든 파일 시스템 테스트는 `t.TempDir()`을 사용한다.
   - `RunDoctor` 테스트는 `DoctorOptions.SessionMgr`(임시 디렉토리 기반 `session.NewManager`)와 `DoctorOptions.LookPath` Mock을 주입해 호스트 `~/.guard/sessions` 및 실제 `PATH`와 완전히 분리한다.
   - CLI E2E 테스트는 `HOME`, `WORKSPACE_ROOT`, `PATH`를 격리하고, `AGENT_MODE`, `AGENT_ROLE`, `GUARD_POLICY_PATH`, `GUARD_SESSION_ID`, `GUARD_LANG` 환경변수를 명시적으로 초기화한다.

---

## 1. 단계별 검증 로드맵

| 단계 | 대상 패키지 / 계층 | 우선순위 | 핵심 검증 목표 |
| :--- | :--- | :---: | :--- |
| **Stage 1** | `internal/hub/setup` (단위) | **P0** | 스택 감지 우선순위, 정책 YAML 유효성, `hooks.json` 비파괴 Safe Merge, Frontmatter 판정·주입 |
| **Stage 2** | `internal/hub/setup` (`RunInit`) | **P0** | 원커맨드 초기화, 멱등성, `--dry-run` 무변경, `--force` `.bak` 백업, `--no-*` 스킵, Fail-Fast |
| **Stage 3** | `internal/hub/setup` (`RunDoctor`) | **P0** | `DOC-001`~`DOC-006` 심각도 판정, `--fix` 비파괴 자동 복구 범위, `ExitCode()` 정확성 |
| **Stage 4** | `internal/adapters/launcher` | **P1** | `EnsureWorkspaceHook`, `EnsureWorkflowRule` 기존 시그니처 호환성 및 `trigger: always_on` 생성 |
| **Stage 5** | `internal/i18n` | **P1** | `ko.json` / `en.json` `cli.init.*`, `cli.doctor.*`, `cli.help` 키 및 포맷 동사 100% 대칭 |
| **Stage 6** | `cmd/guard` (CLI 통합 & E2E) | **P1** | 서브커맨드 플래그/Exit Code, `--json` 순수 JSON 출력, Symlink 정규화, `handoff_*` 회귀 검증 |

---

## Stage 1: Setup 핵심 유틸리티 (`guard/internal/hub/setup`)

### 1.1 스택 감지 (`DetectStack`) 및 프리셋 검증 (`ValidatePreset`)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-INIT-001` | 정상 | 단일 마커 파일 스택 감지 | 각각 `go.mod`, `package.json`, `pyproject.toml`, `requirements.txt`만 존재하는 임시 디렉토리 및 빈 디렉토리 | 순서대로 `("go","go.mod")`, `("node","package.json")`, `("python","pyproject.toml")`, `("python","requirements.txt")`, `("generic","")` 반환 | 마커 파일명 오타, `generic` 폴백 누락 |
| `TC-INIT-002` | 경계 | 다중 마커 공존 시 우선순위 | ① `go.mod`+`package.json`+`pyproject.toml` ② `package.json`+`pyproject.toml` ③ `pyproject.toml`+`requirements.txt` | ① `("go","go.mod")` ② `("node","package.json")` ③ `("python","pyproject.toml")` 반환 | 감지 우선순위 역전 (`go.mod` > `package.json` > `pyproject.toml` > `requirements.txt`) |
| `TC-INIT-003` | 경계 | `ValidatePreset` 허용/거부 및 대소문자 | 허용: `""`, `"auto"`, `"GO"`, `"Node"`, `"python"`, `"generic"`<br>거부: `"rust"`, `"java"`, `"auto "`(또는 무효 문자열) | 허용 값은 `nil`, 거부 값은 `errors.Is(err, setup.ErrInvalidPreset) == true` | 대소문자 미정규화, sentinel error 미래핑 |

### 1.2 정책 YAML 렌더링 (`RenderPolicyYAML`)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-INIT-004` | 정상 | 4종 프리셋 정책 YAML 생성 및 파싱 유효성 | `RenderPolicyYAML`에 `"go"`, `"node"`, `"python"`, `"generic"` 전달 후 임시 파일에 저장해 `policy.LoadFile()` 호출 | 4종 모두 에러 없음, `policy.LoadFile()` 오류·경고 0건, `len(pol.Rules) >= 1` | 템플릿 YAML 문법 오류, 빈 규칙 목록 생성 |
| `TC-INIT-005` | 예외 | 미지원 프리셋으로 정책 렌더링 | `RenderPolicyYAML("rust")`, `RenderPolicyYAML("auto")`, `RenderPolicyYAML("")` | `errors.Is(err, setup.ErrInvalidPreset) == true` (단, 계약상 `go`/`node`/`python`/`generic`만 지원) | 미지원 프리셋에 대한 침묵 폴백 |

### 1.3 `.agents/hooks.json` 검사 (`InspectWorkspaceHook`) 및 Safe Merge (`MergeWorkspaceHook`)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-INIT-006` | 정상 | 미존재 시 신규 생성 및 원자적 쓰기 | 빈 `t.TempDir()`에서 `MergeWorkspaceHook(dir, false)` | `Status == StatusCreated`, `PreservedKeys` 비어 있음, `.agents/hooks.json` 유효 JSON + `guard.PreToolUse`에 `"command": "guard hook"` 존재, 2-space 들여쓰기, `.agents/` 내 `.tmp` 파일 잔존 0건 | 부모 디렉토리 미생성, `.tmp` 임시 파일 누수 |
| `TC-INIT-007` | 정상 | 타 최상위 키 비파괴 병합 (Safe Merge) | `.agents/hooks.json`에 `obsidian-chatlog`(중첩 객체·배열·숫자·불리언 포함) 및 `zeta-tool` 키 존재 시 `MergeWorkspaceHook(dir, false)` | `Status == StatusMerged`, `PreservedKeys == ["obsidian-chatlog", "zeta-tool"]`(정렬), 기존 타 키의 JSON 구조·값이 100% 보존된 채 `guard.PreToolUse` 추가 | 기존 도구 설정 덮어쓰기/손실, `PreservedKeys` 미정렬 |
| `TC-INIT-008` | 경계 | `"guard"` 키 존재하나 `guard hook` 누락 | `{"guard": {"PreToolUse": [{"command": "other-tool"}]}}` 또는 `{"guard": {}}` 상태에서 `MergeWorkspaceHook(dir, false)` | `Status == StatusMerged`, 기존 `"other-tool"` 항목을 유지하면서 `"guard hook"` 추가 | `"guard"` 최상위 키 존재만 보고 `StatusSkipped`로 오판하는 버그 |
| `TC-INIT-009` | 정상 | 이미 등록된 경우 멱등성 | `TC-INIT-006` 이후 `MergeWorkspaceHook(dir, false)` 재호출 | `Status == StatusSkipped`, 파일 바이트 내용 100% 동일 | 중복 hook 엔트리 누적, 불필요한 파일 재쓰기 |
| `TC-INIT-010` | 예외 | 깨진 JSON 보호 (손상 방지) | `.agents/hooks.json`에 `{invalid json`, 빈 파일 `""`, 또는 최상위 배열 `[]` 배치 후 `MergeWorkspaceHook(dir, false)` | `errors.Is(err, setup.ErrInvalidHooksJSON) == true`, **원본 파일 바이트가 1바이트도 변경되지 않음** | 손상된 사용자 설정 파일을 임의로 덮어쓰는 데이터 손실 버그 |
| `TC-INIT-011` | 정상 | `MergeWorkspaceHook` `dryRun: true` 무변경 | ① 미존재 디렉토리 ② 타 키만 존재하는 파일 ③ 이미 등록된 파일에 `dryRun: true` 호출 | ① `StatusPlanned` + `.agents/` 미생성 ② `StatusPlanned` + 파일 바이트 무변경 ③ `StatusSkipped` + 파일 바이트 무변경 | Dry-Run 중 디렉토리 생성 또는 파일 수정 |
| `TC-INIT-012` | 정상 | `InspectWorkspaceHook` 4상태 비파괴 판별 | ① 미존재 ② 깨진 JSON ③ 유효 JSON(`guard` 없음) ④ 유효 JSON(`guard hook` 있음) | ① `(false, false, false, nil)` ② `(true, false, false, ...)` ③ `(true, true, false, nil)` ④ `(true, true, true, nil)` 및 파일 무변경 | 상태 플래그 조합 오판 |

### 1.4 워크플로우 룰 템플릿 및 Frontmatter (`WorkflowRuleContent`, `HasAlwaysOnFrontmatter`, `InjectAlwaysOnFrontmatter`, `EnsureWorkflowRule`)
| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-INIT-013` | 정상 | 기본 워크플로우 룰 템플릿 필수 요소 | `WorkflowRuleContent()` 반환 문자열 검증 | 최상단 `---\ntrigger: always_on\n---`, `<!-- guard-managed: v1 -->` 마커, `guard handoff` CLI 키워드(`inbox`, `status`, `ack`, `send`, `approve` 금지 경계) 포함 | 필수 마커 또는 frontmatter 누락 |
| `TC-INIT-014` | 경계 | `HasAlwaysOnFrontmatter` 정확성 및 오탐 방지 | ① 정상 frontmatter(`trigger: always_on`) ② 다른 키와 공존(`name: x\ntrigger: always_on`) ③ frontmatter 없음 ④ **본문에만** `trigger: always_on` 기재 ⑤ 닫는 `---` 누락 ⑥ `trigger: manual` | ①② `true`, ③④⑤⑥ `false` | 단순 `strings.Contains(content, "trigger: always_on")` 구현으로 인한 본문 오탐 |
| `TC-INIT-015` | 정상 | `InjectAlwaysOnFrontmatter` 3분기 비파괴 주입 | ① 이미 `trigger: always_on` 포함 ② frontmatter 전혀 없는 사용자 문서 ③ 기존 frontmatter(`description: custom`)만 있고 `trigger` 없는 문서 | ① 원본과 100% 동일 ② 상단에 `---\ntrigger: always_on\n---\n\n` 추가 + 본문 보존 ③ 기존 frontmatter 내부에 `trigger: always_on` 삽입 + 기존 키 및 본문 100% 보존, 결과에 `HasAlwaysOnFrontmatter == true` | 기존 frontmatter 중복 생성 또는 사용자 본문 손실 |
| `TC-INIT-016` | 정상 | `EnsureWorkflowRule` 생성 및 기존 파일 보존 | ① 빈 워크스페이스 ② 커스텀 내용의 `.agents/rules/guard-workflow.md` 존재 | ① `created == true`, 파일 생성 및 `trigger: always_on` 포함 ② `created == false`, 기존 파일 바이트 100% 보존 | 기존 사용자 룰 파일 덮어쓰기 |

---

## Stage 2: `RunInit` 오케스트레이션 (`guard/internal/hub/setup`)

| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-INIT-017` | 정상 | 빈 Go 프로젝트 기본 초기화 | `go.mod`만 있는 임시 디렉토리에 `RunInit(InitOptions{WorkspaceRoot: dir})` | `DetectedPreset == "go"`, `DetectedMarker == "go.mod"`, `DryRun == false`; 4개 파일(`.agents/hooks.json`, `guard-policy.yaml`, `.agents/rules/guard-workflow.md`, `.agents/current_mode`) 생성; `current_mode == "DEV"` | 4개 중 일부 파일 누락, 기본 모드 미설정 |
| `TC-INIT-018` | 정상 | 명시적 프리셋 및 모드 정규화 | `go.mod` 존재 디렉토리에 `Preset: "python"`, `Mode: "test"` (소문자) 전달 | `DetectedPreset == "python"`, `guard-policy.yaml`이 python 프리셋으로 생성, `.agents/current_mode`에 `"TEST"` 기록 | 명시적 프리셋 무시(자동 감지로 덮임), 모드 대문자 미정규화 |
| `TC-INIT-019` | 예외 | 잘못된 Preset / Mode 입력 시 Fail-Fast | ① `Preset: "invalid"` ② `Mode: "ADMIN"` | 각각 `ErrInvalidPreset`, `ErrInvalidMode` 반환; **어떠한 파일/디렉토리(`.agents/` 포함)도 생성되지 않음** | 검증 전 일부 파일이 먼저 생성되는 부분 초기화 버그 |
| `TC-INIT-020` | 정상 | 2회 연속 실행 멱등성 (Idempotency) | 동일 디렉토리에서 `RunInit` 2회 연속 실행 | 2회차 `Steps`의 모든 항목이 `StatusSkipped`; 4개 파일의 바이트 내용이 1회차와 100% 동일 | 재실행 시 파일 덮어쓰기 또는 `StatusCreated` 오보고 |
| `TC-INIT-021` | 정상 | `--dry-run` 완전 무변경 (빈 디렉토리 & 기존 파일) | ① 빈 `go.mod` 디렉토리 ② 일부 파일이 이미 존재하는 디렉토리에 `DryRun: true` | `result.DryRun == true`; ① `.agents/` 및 `guard-policy.yaml` 일체 미생성, 전 항목 `StatusPlanned` ② 기존 파일은 `StatusSkipped`, 미존재 파일은 `StatusPlanned`, 디스크 변경 0건 | `--dry-run`에서 디렉토리 생성 또는 파일 수정 발생 |
| `TC-INIT-022` | 정상 | `--force` `.bak` 백업 무손실 재생성 | 커스텀 내용의 `guard-policy.yaml`과 `.agents/rules/guard-workflow.md` 존재 시 `Force: true` 실행 | 두 항목 모두 `StatusBackedUp` 및 `BackupPath`(`.bak`) 반환; `.bak` 파일 내용이 기존 커스텀 내용과 100% 일치; 본 파일은 최신 템플릿으로 재생성 | 백업 없이 덮어쓰기, `BackupPath` 누락 |
| `TC-INIT-023` | 정상 | 선택 스킵 옵션 (`NoHooks`, `NoPolicy`, `NoRules`) | ① `NoHooks: true` ② `NoPolicy: true` ③ `NoRules: true` ④ 셋 모두 `true` | 스킵 지정된 대상은 파일 미생성(또는 스킵 처리), 지정되지 않은 대상만 정상 생성 | 스킵 플래그 무시 또는 서로 다른 플래그 혼선 |
| `TC-INIT-024` | 예외 | 깨진 `.agents/hooks.json` 존재 시 `RunInit` | 손상된 `.agents/hooks.json` 배치 후 `RunInit` 실행 | `errors.Is(err, setup.ErrInvalidHooksJSON) == true`, 원본 `hooks.json` 바이트 무변경 | 에러 무시 또는 손상 파일 덮어쓰기 |
| `TC-INIT-025` | 경계 | 기존 `.agents/current_mode` 존재 시 보존 | `.agents/current_mode`에 `"PLAN"` 저장된 상태에서 `RunInit(Mode: "DEV")` | `current_mode` 단계 `StatusSkipped`, 파일 내용 `"PLAN"` 그대로 보존 | 기존 활성 모드를 기본값(`DEV`)으로 덮어쓰는 버그 |

---

## Stage 3: `RunDoctor` 진단 및 자동 복구 (`DOC-001`~`DOC-006`)

| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-DOC-001` | 정상 | 완전히 건강한 워크스페이스 진단 | `RunInit` 완료 + 정상 `LookPath` mock + 정상 `SessionMgr` 디렉토리 + canonical `WorkspaceRoot` | `len(Checks) == 6` (`DOC-001`~`DOC-006` 순서), 전부 `SeverityOK`, `ErrorCount == 0`, `WarnCount == 0`, `FixedCount == 0`, `ExitCode() == 0` | 정상 환경에서 오경보(False Positive) |
| `TC-DOC-002` | 예외 | `DOC-001`: `PATH`에서 `guard` 미발견 | `LookPath`가 `exec.ErrNotFound` 반환 (`Fix: false` 및 `Fix: true`) | `DOC-001`: `SeverityError`, `Fixable == false`, `Fixed == false`, `ErrorCount >= 1`, `ExitCode() == 1` (`Fix: true`에서도 동일) | `DOC-001`을 `WARN` 또는 `Fixable: true`로 오분류 |
| `TC-DOC-003` | 정상 | `DOC-002`: `hooks.json` 미존재 / `guard hook` 누락 → 자동 복구 | ① `.agents/hooks.json` 미존재 ② 타 키(`obsidian-chatlog`)만 존재 → `Fix: false` 진단 후 `Fix: true` 실행 후 재진단 | 진단: `SeverityError`, `Fixable == true`, `ExitCode() == 1`<br>`Fix: true`: `SeverityFixed`, `Fixed == true`, `FixedCount >= 1`, 타 키 100% 보존<br>재진단: `DOC-002 == SeverityOK` | 복구 후 `Fixed` 미표시, 타 키 손실 |
| `TC-DOC-004` | 예외 | `DOC-002`: 깨진 `hooks.json`은 `--fix`로도 덮어쓰지 않음 | `.agents/hooks.json`에 `{broken` 배치 후 `Fix: true`로 `RunDoctor` | `DOC-002`: `SeverityError`, `Fixed == false`, `ExitCode() == 1`, **원본 파일 바이트 `{broken` 100% 무변경** | `--fix`가 깨진 JSON을 강제 덮어쓰는 치명적 결함 |
| `TC-DOC-005` | 정상 | `DOC-003`: 정책 파일 미존재 (`builtin` fallback) → 자동 복구 | `go.mod`만 있고 `guard-policy.yaml` 미존재 → `Fix: false` 후 `Fix: true` 후 재진단 | 진단: `DOC-003 == SeverityWarn`, `Fixable == true`, `ExitCode() == 0` (다른 Error 없을 시)<br>`Fix: true`: `go` 프리셋 정책 생성 + `SeverityFixed`<br>재진단: `DOC-003 == SeverityOK` | `builtin` fallback을 `ERROR`로 오판하거나 `--fix` 미생성 |
| `TC-DOC-006` | 예외 | `DOC-003`: 기존 `guard-policy.yaml` 문법 오류 / 빈 규칙 → 보호 | ① 문법이 깨진 `guard-policy.yaml` ② `rules: []` 정책 배치 후 `Fix: true` | `DOC-003`: `SeverityError`, `Fixed == false`, `ExitCode() == 1`, **원본 정책 파일 바이트 100% 무변경** | 사용자가 편집 중인 잘못된 정책 파일을 `--fix`가 덮어쓰는 결함 |
| `TC-DOC-007` | 정상 | `DOC-004`: `guard-workflow.md` 미존재 → 자동 생성 | `.agents/rules/guard-workflow.md` 미존재 → `Fix: false` 후 `Fix: true` | 진단: `SeverityWarn`, `Fixable == true`<br>`Fix: true`: 파일 생성(`trigger: always_on` 포함), `SeverityFixed`<br>재진단: `SeverityOK` | 파일 미생성 또는 `trigger: always_on` 누락 |
| `TC-DOC-008` | 정상 | `DOC-004`: `trigger: always_on` 누락 파일 비파괴 복구 | ① frontmatter 없는 사용자 커스텀 룰 ② 기존 frontmatter(`name: custom`)만 있는 룰 → `Fix: false` 후 `Fix: true` 후 재진단 | 진단: `SeverityWarn`, `Fixable == true`<br>`Fix: true`: `SeverityFixed`, **사용자 본문 100% 보존** + 상단 `trigger: always_on` 삽입<br>재진단: `DOC-004 == SeverityOK` | `--fix`가 사용자 룰 파일을 기본 템플릿으로 덮어쓰는 결함 |
| `TC-DOC-009` | 정상 | `DOC-005`: 세션 디렉토리 미존재 및 Stale PID 정리 | ① 존재하지 않는 세션 디렉토리 ② 살아있는 PID(`os.Getpid()`) 1건 + 죽은 PID(`999999`) 1건 + `PID: 0` 1건 배치 → `Fix: false` 후 `Fix: true` | 진단: `SeverityWarn`, `Fixable == true`<br>`Fix: true`: ① 디렉토리 생성 + `SeverityFixed` ② Stale 2건 삭제 + **살아있는 세션 1건 보존** + `SeverityFixed`<br>재진단: `DOC-005 == SeverityOK` | 살아있는 활성 세션까지 삭제, 디렉토리 미생성 |
| `TC-DOC-010` | 예외 | `DOC-005`: 세션 디렉토리 쓰기 권한 없음 | 세션 디렉토리를 읽기 전용(`0555`)으로 설정(단, `os.Geteuid() == 0`이면 skip) | `DOC-005`: `SeverityError`, `ExitCode() == 1` | 쓰기 불가 상태를 `OK` 또는 `WARN`으로 오판 |
| `TC-DOC-011` | 경계 | `DOC-006`: `WorkspaceRoot`가 Symlink 경로인 경우 | 실제 디렉토리 `realDir`을 가리키는 심볼릭 링크 `linkDir`을 `WorkspaceRoot`로 전달 (`Fix: false` 및 `Fix: true`) | `DOC-006`: `SeverityWarn`, `Fixable == false`, `Fixed == false`; 다른 Error가 없으면 `ExitCode() == 0` | 심볼릭 링크 불일치 미감지 또는 `ERROR`로 오판 |
| `TC-DOC-012` | 경계 | `DoctorReport` 카운터 집계 및 `ExitCode()` 경계 | ① `WARN` 3건 + `ERROR` 0건 ② `WARN` 0건 + `ERROR` 1건 ③ `FIXED` 2건 + `ERROR` 0건 | ① `WarnCount == 3`, `ErrorCount == 0`, `ExitCode() == 0` ② `ErrorCount == 1`, `ExitCode() == 1` ③ `FixedCount == 2`, `ErrorCount == 0`, `ExitCode() == 0` | `WARN`만 있는데 Exit Code `1` 반환, `FixedCount`가 `WarnCount`/`ErrorCount`에 중복 집계 |

---

## Stage 4: Launcher Bootstrap 호환성 (`guard/internal/adapters/launcher`)

| ID           | 구분  | 시나리오                                              | 입력 조건                                                   | 기대 결과 (Assertion)                                                                                             | 결함 탐지 포인트                                           |
| :----------- | :-: | :------------------------------------------------ | :------------------------------------------------------ | :------------------------------------------------------------------------------------------------------------ | :-------------------------------------------------- |
| `TC-LCH-001` | 정상  | `EnsureWorkspaceHook` 생성 · 병합 · 멱등성               | ① 빈 디렉토리 ② `obsidian-chatlog` 키 존재 ③ 이미 `guard hook` 존재 | ① `updated == true`, 파일 생성 ② `updated == true`, 타 키 100% 보존 ③ `updated == false`, 파일 무변경                      | `setup.MergeWorkspaceHook` 위임 시 `updated` 반환 의미 불일치 |
| `TC-LCH-002` | 예외  | `EnsureWorkspaceHook` 깨진 JSON 보호                  | `.agents/hooks.json`에 `{broken` 배치 후 호출                 | `err != nil`, `updated == false`, 원본 파일 바이트 무변경                                                               | 손상 JSON 덮어쓰기                                        |
| `TC-LCH-003` | 정상  | `EnsureWorkflowRule` 생성 시 `trigger: always_on` 포함 | 빈 디렉토리에서 `EnsureWorkflowRule(ws)` 호출                    | `created == true`, `rulePath` 생성, `setup.HasAlwaysOnFrontmatter(content) == true`, `WorkflowManagedMarker` 포함 | 구버전 템플릿(frontmatter 없음) 잔존                          |
| `TC-LCH-004` | 정상  | `EnsureWorkflowRule` 기존 파일 보존                     | 커스텀 `.agents/rules/guard-workflow.md` 존재 시 호출           | `created == false`, 기존 파일 바이트 100% 보존                                                                         | 기존 사용자 룰 덮어쓰기                                       |

---

## Stage 5: i18n 대칭성 (`guard/internal/i18n`)

| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-I18N-001` | 정상 | `ko.json` / `en.json` 키 집합 100% 대칭 및 신규 키 존재 | 내장 `ko.json`과 `en.json` 전체 키 비교 + `cli.help`, `cli.init.*`, `cli.doctor.*` 접두 키 존재 확인 | 양쪽 키 집합 차집합 0건, `cli.init.*` 및 `cli.doctor.*` 키가 양쪽 모두 비어있지 않은 문자열로 존재 | 한쪽 로케일에만 키 누락, 빈 번역 문자열 |
| `TC-I18N-002` | 정상 | 포맷 동사(`%s`, `%d`, `%v` 등) 일치성 | `ko.json`과 `en.json`의 모든 대응 키에서 `fmt` 포맷 동사 목록/개수 비교 | 모든 키에서 포맷 동사 개수 및 타입 일치 (`%!s(MISSING)` / `%!(EXTRA ...)` 방지) | 번역문 포맷 인자 불일치로 인한 런타임 출력 깨짐 |

---

## Stage 6: CLI 통합 및 E2E (`guard/cmd/guard`)

| ID | 구분 | 시나리오 | 입력 조건 | 기대 결과 (Assertion) | 결함 탐지 포인트 |
| :--- | :---: | :--- | :--- | :--- | :--- |
| `TC-E2E-001` | 정상 | `guard help` / `guard --help` | 빌드된 `guard` 바이너리에 `help`, `--help` 실행 | Exit `0`, stdout에 `guard init` 및 `guard doctor` 항목 포함 | 도움말 문서 누락 |
| `TC-E2E-002` | 정상 | `guard init --dry-run` → `guard init` → 재실행 E2E | 격리 `HOME`, `WORKSPACE_ROOT`(`go.mod` 포함)에서 순차 실행 | `--dry-run`: Exit `0`, `.agents/` 미생성<br>1회 `init`: Exit `0`, 4개 파일 생성<br>2회 `init`: Exit `0`, 4개 파일 바이트 100% 동일 | CLI 플래그 파싱 오류, 멱등성 붕괴 |
| `TC-E2E-003` | 정상 | `guard init --force` 및 `--no-*` CLI 옵션 | 커스텀 정책 파일 존재 시 `--force` 실행; 빈 디렉토리에서 `--no-hooks --no-rules` 실행 | `--force`: Exit `0`, `guard-policy.yaml.bak` 원본 보존<br>`--no-*`: Exit `0`, 스킵 대상 미생성 | CLI 플래그 → `InitOptions` 매핑 누락 |
| `TC-E2E-004` | 예외 | `guard init` 잘못된 인자 Fail-Fast | ① `guard init --unknown` ② `guard init extra-arg` ③ `guard init --preset rust` ④ `guard init --mode BOGUS` | 모두 Exit `1`, stderr 비어있지 않음, 파일 시스템 변경 0건 | 잘못된 플래그/positional args 무시 |
| `TC-E2E-005` | 정상 | `guard doctor` 정상 / 오류 / `--fix` 흐름 | ① 미초기화 워크스페이스에서 `guard doctor` ② `guard doctor --fix` ③ `guard doctor` 재실행 (`PATH`에 빌드 바이너리 포함) | ① Exit `1` (`DOC-002` ERROR) ② Exit `0` (복구 완료) ③ Exit `0` (전체 OK) | Exit Code 반전, `--fix` 후 상태 미갱신 |
| `TC-E2E-006` | 정상 | `guard doctor --json` 순수 JSON 출력 계약 | 정상 환경 및 오류 환경에서 `guard doctor --json` 실행 | stdout 전체가 `json.Unmarshal`로 `setup.DoctorReport`에 오류 없이 파싱됨(배너·일반 텍스트 혼합 0건), `len(Checks) == 6`, 오류 환경에서는 Exit `1` + 유효 JSON | stdout에 배너/로그가 섞여 CI 파이프라인 파싱 실패 |
| `TC-E2E-007` | 예외 | `guard doctor` 잘못된 인자 Fail-Fast | ① `guard doctor --unknown` ② `guard doctor extra-arg` | 모두 Exit `1`, stderr 오류 메시지 출력 | 미지원 인자 무시 |
| `TC-E2E-008` | 정상 | Symlink `WORKSPACE_ROOT` 정규화 (`guard status` / `handoff`) | `realDir`과 이를 가리키는 심볼릭 링크 `linkDir` 생성 후 `WORKSPACE_ROOT=linkDir`로 `guard status` 및 `guard handoff` 실행 | 출력 워크스페이스 경로 및 핸드오프 라우팅이 `realDir`(canonical path)로 정규화되어 실제 경로 실행과 동일하게 동작 | `findWorkspaceRoot()` symlink 미정규화로 인한 세션/핸드오프 불일치 |
| `TC-E2E-009` | 회귀 | 기존 `test/e2e/handoff_*` 회귀 검증 | `go test ./test/e2e/...` 실행 | 기존 핸드오프 E2E 시나리오 전체 PASS (회귀 0건) | `findWorkspaceRoot()` 및 `launcher` 변경으로 인한 기존 기능 회귀 |

---

## 2. 수용 기준(AC) ↔ 테스트 케이스 추적성 매트릭스

| 수용 기준 ([init-automation-qa-test-brief.md §6](./init-automation-qa-test-brief.md#6-최상위-수용-기준-acceptance-criteria)) | 매핑된 테스트 케이스 |
| :--- | :--- |
| **AC-1**: 스택 자동 감지 우선순위 & 4종 프리셋 정책 유효성 | `TC-INIT-001` ~ `TC-INIT-005`, `TC-INIT-017`, `TC-INIT-018` |
| **AC-2**: Safe Merge(타 키 100% 보존) & 깨진 JSON 보호 | `TC-INIT-006` ~ `TC-INIT-012`, `TC-INIT-024`, `TC-DOC-003`, `TC-DOC-004`, `TC-LCH-001`, `TC-LCH-002` |
| **AC-3**: Idempotent / Dry-Run 무변경 / Force `.bak` 백업 | `TC-INIT-011`, `TC-INIT-019` ~ `TC-INIT-023`, `TC-INIT-025`, `TC-E2E-002` ~ `TC-E2E-004` |
| **AC-4**: Always-On 룰 주입 & `DOC-004` 비파괴 복구 | `TC-INIT-013` ~ `TC-INIT-016`, `TC-DOC-007`, `TC-DOC-008`, `TC-LCH-003`, `TC-LCH-004` |
| **AC-5**: Doctor `DOC-001`~`DOC-006` 정확성, `ExitCode()`, `--json` 순수 출력 | `TC-DOC-001` ~ `TC-DOC-012`, `TC-E2E-005` ~ `TC-E2E-007` |
| **AC-6**: Symlink 정규화, 기존 `handoff_*` 회귀 방지 & i18n 대칭성 | `TC-DOC-011`, `TC-I18N-001`, `TC-I18N-002`, `TC-E2E-001`, `TC-E2E-008`, `TC-E2E-009` |
