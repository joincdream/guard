# QA Phase 1~2 블랙박스 테스트 결과 및 검토 요청 보고서

- **작성일**: 2026-11-20
- **작성자**: QA
- **기준 시나리오**: [test-scenario-plan.md](./test-scenario-plan.md) (`Phase 1` ~ `Phase 2`)
- **테스트 환경**: Linux / Go Test (`-race` 포함) / guard `TEST` 모드

---

## 1. 실행 요약

| 단계 | 패키지 | 실행 명령 | 결과 | 소요 시간 | 비고 |
| :--- | :--- | :--- | :---: | :---: | :--- |
| **Phase 1** | `guard/internal/core` | `go test -v -count=1 ./internal/core/...` | **PASS** | 0.007s | 기존 테스트 + `TC-P1-001`~`025` 전체 통과 |
| **Phase 2** | `guard/internal/hub/policy` | `go test -v -race -count=1 ./internal/hub/policy/...` | **PASS** | 1.026s | 기존 테스트 + `TC-P2-001`~`007` 전체 통과 (레이스 0건) |
| **Phase 2** | `guard/internal/hub/session` | `go test -v -race -count=1 ./internal/hub/session/...` | **FAIL** | 0.395s | `TC-P2-015`(`-race` 동시성) 포함 11건 통과 / **1건 실패 (`DEF-003`)** |

---

## 2. 발견 이슈 상세 (`DEF-003`)

### DEF-003: `SessionPath` 단순 `_` 치환으로 인해 서로 다른 세션 ID 간 파일명 충돌(Silent Overwrite) 발생

- **심각도 / 우선순위**: Low~Medium / P3 (비표준 특수문자 포함 세션 ID 동시 사용 시 세션 덮어쓰기 발생)
- **대상 패키지**: `guard/internal/hub/session`
- **관련 테스트**: `internal/hub/session/blackbox_phase2_test.go` (`L103-L152`, `TestPhase2_Session_IDSanitizationAndHiddenFileDefense` / `TC-P2-010`)
- **단독 재현 명령**:
  ```bash
  go test -v -race -count=1 ./internal/hub/session -run TestPhase2_Session_IDSanitizationAndHiddenFileDefense
  ```

#### 재현 절차
1. 단일 세션 디렉토리에 서로 다른 5개 ID로 `mgr.Create(...)` 순차 호출:
   - `"../evil"`, `"a/b"`, `"a\b"`, `".hidden"`, `"..hidden"`
2. 상위 디렉토리 탈출 여부 및 숨김 파일(`.`) 생성 여부 검증.
3. `mgr.List()` 호출 후 생성한 5개 세션 ID가 모두 존재하는지 확인.

#### 기대 결과 (Expected Result)
- 5개 ID 모두 세션 디렉토리 내부에 안전하게 저장(Path Traversal 없음, 선행 `.` 없음).
- 서로 다른 5개의 세션 ID가 `List()` 결과에 모두 존재.

#### 실제 결과 (Actual Result)
- Path Traversal 방어 및 숨김 파일 방어 검사는 **모두 통과**.
- 그러나 `List()` 결과에는 `../evil`, `..hidden`, `a\b` **3건만 존재**하고, 먼저 생성된 `"a/b"`와 `".hidden"` **2건이 유실**됨.
- 에러 로그:
  ```text
  blackbox_phase2_test.go:148: session "a/b" missing from List() (found: map[../evil:true ..hidden:true a\b:true])
  blackbox_phase2_test.go:148: session ".hidden" missing from List() (found: map[../evil:true ..hidden:true a\b:true])
  ```

#### 원인 분석
계약서(`doc/spec/02_session_hub_contract.md` §2 `SessionPath`)의 치환 규칙(`/`, `\`, `..`, 선행 `.` → `_`)에 따라 서로 다른 원본 ID 쌍이 동일한 파일명으로 매핑되면서 에러 없이 기존 세션을 덮어쓰는 **ID Collision (Silent Overwrite)**이 발생했습니다.
- `"a/b"` 및 `"a\b"` → 둘 다 `a_b.json` (나중에 생성된 `"a\b"`가 `"a/b"`를 덮어씀)
- `".hidden"` 및 `"..hidden"` → 둘 다 `_hidden.json` (나중에 생성된 `"..hidden"`이 `".hidden"`을 덮어씀)

---

## 3. 개발자 검토 및 결정 요청 사항

`DEF-003`은 계약된 단순 치환 규칙의 구조적 충돌에 해당합니다. 아래 두 방향 중 제품 정책에 맞는 방향을 회신해 주시기 바랍니다.

- [ ] **Option A (프로덕션 개선)**: 서로 다른 세션 ID가 동일 파일명으로 충돌하여 덮어쓰지 않도록 `sanitizeID`에 충돌 방지 인코딩/해시 접미사를 도입하거나, 파일명 충돌 시 에러를 반환하도록 개선.
- [ ] **Option B (현 스펙 유지)**: 실제 운영 세션 ID는 UUID/영숫자 기반이므로 단순 `_` 치환 스펙을 유지. 이 경우 QA에서 `TC-P2-010`을 각 ID별 독립 디렉토리(`t.TempDir()`) 서브테스트로 분리하여 Path Traversal 및 숨김 파일 방어 계약만 검증하도록 조정.

---

## 4. 참고 — 정상 검증 완료된 주요 항목 (Phase 1~2)
- `internal/core`: 모드 정규화/Bypass, 패턴 경계 오탐 방지, `ExcludePatterns`, 규칙 우선순위, 셸 제어 연산자·리다이렉션·Git·뷰어·쓰기 도구·인라인 스크립트 우회 차단 전수 통과
- `internal/hub/policy`: 5단계 우선순위 폴백, 손상 YAML 및 `rules: []`(0건) 경고 누적, `ResolveWorkspaceMode` 4단계 폴백 전수 통과
- `internal/hub/session`: 잘못된 `Create` 입력 방어, `List()` 정렬/손상 파일 내성, `Delete`/`DeleteAll` `.lock` 정리, 50/51/100건 이벤트 로테이션, `PID <= 0` GC, **20고루틴×5회 동시 `Flock` 쓰기(`-race`) Lost Update 0건** 통과

---

## 5. 개발자 검토 및 수정 회신 (2026-11-20)

### `DEF-003` — Option A (하이브리드 충돌 방지) 적용 완료
- **수정 파일**: `internal/hub/session/manager.go` (`sanitizeID`), `doc/spec/02_session_hub_contract.md`
- **수정 내용**:
  - 정상 ID(UUID, 8자리 hex 등 치환이 발생하지 않는 ID)는 기존 파일명을 100% 원문 그대로 유지합니다.
  - `/`, `\`, `..`, 선행 `.` 치환이 발생한 비표준 ID에 한해 원본 ID의 결정적 8자리 SHA-256 hex 접미사(`_<8hex>`)를 부여하여, 서로 다른 비표준 ID(`"a/b"` vs `"a\b"`, `".hidden"` vs `"..hidden"`) 간 파일명 충돌 및 Silent Overwrite를 원천 차단했습니다.
- **QA 확인 요청**: `TestPhase2_Session_IDSanitizationAndHiddenFileDefense` (`TC-P2-010`) 및 `internal/hub/session` 전체 `-race` 재검증을 부탁드립니다.

---

## 6. QA 재검증 및 종료 보고 (2026-11-20)

- **재검증 명령**: `go test -v -race -count=1 ./internal/hub/session/...`
- **재검증 결과**: **PASS** (`ok guard/internal/hub/session 1.401s`, 종료 코드 `0`, 데이터 레이스 0건)
  - `TestPhase2_Session_IDSanitizationAndHiddenFileDefense` (`TC-P2-010`) **PASS** (테스트 코드 수정 없이 통과)
  - 기존 3건 + Phase 2 블랙박스 9건(`TC-P2-008` ~ `TC-P2-016`) 전체 12건 **PASS**
- **결함 최종 상태**:
  - **DEF-003**: **Closed** (Option A 프로덕션 수정 검증 완료)
- **Phase 1~2 최종 상태**: `core`, `hub/policy`, `hub/session` 전수 통과 완료
