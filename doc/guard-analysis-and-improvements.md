# Guardrail (guard) 분석 및 개선 계획 보고서

## 1. 개요 및 목적
`guard`는 **Antigravity CLI (agy)**의 `PreToolUse` 훅(Hook)과 연동하여 AI 에이전트의 불필요하거나 정책에 위배되는 파일 접근(읽기/쓰기) 및 셸 명령어 실행을 차단하고 통제하는 가드레일 프로그램입니다.

본 문서는 현재 구현된 코드베이스(`main.go`, `guard-policy.yaml`, `Makefile`)에 대한 분석 내용과 향후 적용 가능한 개선 포인트를 정리한 문서입니다.

---

## 2. 현재 시스템 분석

### 2.1 아키텍처 및 동작 모드
`guard`는 단일 Go 바이너리로 빌드되며, 실행 환경에 따라 두 가지 방식으로 동작합니다.

1. **CLI 서브커맨드 모드 (관리 및 제어)**
   - 인자가 제공될 때 실행됩니다.
   - 서브커맨드:
     - `guard dev`: DEV 모드 전환 (테스트 파일 열람 차단)
     - `guard test`: TEST 모드 전환 (프로덕션 코드 열람 차단)
     - `guard plan`: PLAN 모드 전환 (프로덕션 소스 코드 수정 차단)
     - `guard all`: ALL 모드 전환 (모든 차단 해제)
     - `guard status`: 현재 활성 모드, 워크스페이스, 정책 파일 로드 상태 출력
     - `guard run <mode> <cmd...>`: 특정 `AGENT_MODE` 환경변수를 주입하여 하위 프로세스 실행
   - 모드 전환 시 `.agents/current_mode` 및 세션 임시 파일(`/tmp/aicc-session-<id>.mode`)에 상태를 저장합니다.

2. **Hook 모드 (agy 자동 인터셉트)**
   - 인자 없이 agy의 PreToolUse 훅에 의해 stdin으로 JSON 페이로드를 전달받아 실행됩니다.
   - 출력 규격:
     - 허용: `{"decision": "allow"}` (Exit code 0)
     - 차단: `{"decision": "deny", "reason": "<메시지>"}` (Exit code 0, stderr에 상세 사유 출력)

### 2.2 모드 판별 우선순위
`determineMode` 함수는 다음 순서로 활성 모드를 결정합니다 (기본값: `DEV`):
1. 세션 임시 파일 (`/tmp/aicc-session-<SessionID>.mode` 또는 `.role`)
2. 환경변수 (`AGENT_MODE`, `AGENT_ROLE`)
3. 워크스페이스 내 설정 파일 (`.agents/current_mode`)

### 2.3 정책 정의 (`guard-policy.yaml`)
| 모드 (Mode) | 행위 (Type) | 대상 패턴 (Patterns) | 예외 패턴 (Exclude) | 목적 |
| :--- | :--- | :--- | :--- | :--- |
| **DEV** | `read` | `*_test.go`, `*.test.ts`, `**/tests/**`, `**/000/**` 등 | 없음 | 클린룸 개발: 테스트 코드를 미리 열람하여 하드코딩하거나 치팅하는 행위 차단 |
| **PLAN** | `write` | `src/internal/**`, `src/cmd/**`, `src/web/**`, `src/docker/**` | 없음 | 설계 단계: 소스 코드 및 인프라 파일 임의 수정 차단, 문서만 작성 유도 |
| **TEST** | `read` | `src/internal/**`, `src/cmd/**` | `*_test.go`, `**/testutil/**`, `**/contract/**` 등 | TDD 테스트 작성: 내부 구현 은닉, 인터페이스/계약 기반 테스트 작성 유도 |

### 2.4 인터셉트 및 검증 로직
1. **도구 수준 검증 (`Tool-level`)**
   - 대상 도구: `read_file`, `write_file`, `edit_file`
   - 페이로드의 `Path` 인자를 추출하여 정책 규칙의 패턴 및 예외 패턴과 매칭 검사
   - Glob, 디렉터리 접두사, `**` 이중 와일드카드 정규식 변환 지원
2. **명령어 수준 검증 (`Command-line`)**
   - 대상 도구: `run_command`
   - 안전 명령어 화이트리스트(`git status`, `git branch`, `go build`, `go vet` 등) 사전 통과
   - DEV 모드: 뷰어 도구(`cat`, `head`, `tail` 등), 와일드카드 전개, Git 검사(`git show`, `git diff`), 인라인 스크립트(`python -c`, `node -e`)를 통한 테스트 파일 열람 시도 차단
   - PLAN 모드: 리다이렉션(`>`, `>>`), `tee`, `sed -i`를 통한 소스 코드 수정 시도 차단

---

## 3. 개선 분석 및 개선 과제

### 3.1 정책 파일 탐색 경로 유연화 (Configuration Path Lookup)
- **현상**: `loadPolicy`가 `<workspaceRoot>/.agents/guard-policy.yaml` 경로만 고정 조회하며, 파일이 없으면 하드코딩된 폴백 규칙으로 전환됨. 현재 작업 디렉터리에 위치한 `guard-policy.yaml`이 바로 반영되지 않을 수 있음.
- **개선안**:
  1. 환경변수 `GUARD_POLICY_PATH` 지원
  2. 현재 작업 디렉터리의 `./guard-policy.yaml` 조회
  3. `<workspaceRoot>/.agents/guard-policy.yaml` 조회
  4. 실행 파일 위치 및 홈 디렉터리(`~/.config/guard/guard-policy.yaml`) 순차 탐색 후 폴백

### 3.2 셸 명령어 파싱 및 우회 방어 강화 (Command Parsing Robustness)
- **현상**: 현재 셸 명령어 분석은 단순 파이프(`|`) 분리와 접두사/문자열 포함 검사에 의존하고 있음.
- **개선안**:
  1. 복합 제어 연산자 처리: 세미콜론(`;`), `&&`, `||`, 서브셸(`$(...)`, `` `...` ``) 분리 분석
  2. 절대/상대 경로 명령어 인식: `/bin/cat`, `/usr/bin/head` 등의 경로 처리
  3. 셸 에일리어스 및 환경변수 치환 우회 방지: `sh -c "..."`, `bash -c "..."` 구문 재귀 검사

### 3.3 검사 대상 도구 확장 (Tool Coverage)
- **현상**: `read_file`, `write_file`, `edit_file`, `run_command`에 집중되어 있음.
- **개선안**:
  - agy 및 IDE 환경에서 추가로 사용될 수 있는 파일 검색/조회 도구(`grep_search`, `file_search`, `list_directory`)에 대한 정책 규칙 적용 고려

### 3.4 테스트 및 검증 체계 구축
- **현상**: `Makefile`에 `test: go test -v ./...` 타깃이 있으나 테스트 코드(`main_test.go`)가 아직 작성되지 않음.
- **개선안**:
  - `matchPattern`, `evaluatePathRules`, `evaluateCommandRules`에 대한 단위 테스트 작성
  - 실제 agy Hook JSON 페이로드를 가상으로 주입하는 E2E 테스트 케이스 추가

---

## 4. 파일 링크
- 정책 설정 파일: [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml)
- 가드레일 메인 코드: [main.go](file:///mnt/data/mytask/kms/guard/main.go)
- 빌드 스크립트: [Makefile](file:///mnt/data/mytask/kms/guard/Makefile)
