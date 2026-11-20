# [06_i18n_design.md] Guard CLI/TUI 다국어(i18n) 상세 설계서

## 1. 개요 및 목적 (Overview)

### 1.1 배경
`guard`는 단일 Go 바이너리로 배포되는 CLI 도구이자, 매 도구 호출마다 실행되는 Antigravity `PreToolUse` 훅 어댑터입니다. 현재 사용자 대면 메시지(CLI 도움말, `status`, 런처 배너, `guard top` TUI, 내장 폴백 정책 메시지)가 한국어로 하드코딩되어 있어 다국어(영어 등) 환경 지원이 어렵습니다.

### 1.2 설계 원칙
- **단일 바이너리 유지**: `//go:embed`를 사용해 번역 카탈로그를 바이너리에 내장 (런타임 디스크 I/O 없음, 훅 지연 0).
- **외부 의존성 최소화**: Go 표준 라이브러리(`embed`, `encoding/json`, `fmt`)만 사용.
- **안전한 폴백**: 번역 키 누락 시에도 패닉이나 빈 문자열이 발생하지 않도록 `현재 언어 -> en -> 키 원문` 순으로 폴백.
- **프로토콜 불변성 보장**: 훅 JSON 프로토콜 키 및 모드 식별자는 번역 대상에서 제외.

---

## 2. 패키지 구조 (`internal/i18n`)

```text
internal/i18n/
├── i18n.go            # Init(), SetLang(), Lang(), T(key, args...), RelativeTime(t)
├── detect.go          # 언어 자동 감지 (GUARD_LANG -> LC_ALL -> LC_MESSAGES -> LANG)
├── i18n_test.go       # 키 정합성, 포맷 동사(%s/%d) 일치, 로케일 감지 단위 테스트
└── locales/
    ├── en.json        # 기본(Fallback) 영어 카탈로그 (//go:embed)
    └── ko.json        # 한국어 카탈로그 (//go:embed)
```

---

## 3. 언어 감지 우선순위

애플리케이션 시작 시 `i18n.Init()`이 아래 우선순위로 활성 언어를 결정합니다.

1. **CLI 플래그 (선택 확장)**: `--lang=ko|en`
2. **전용 환경변수**: `GUARD_LANG` (예: `GUARD_LANG=en guard top`)
3. **표준 POSIX 로케일 환경변수**: `LC_ALL` → `LC_MESSAGES` → `LANG`
   - 접두사 파싱 규칙: `ko_KR.UTF-8` → `ko`, `en_US.UTF-8` → `en`
   - `C`, `POSIX` 또는 미지원 로케일 → 기본 폴백 언어로 처리
4. **기본 폴백 언어**: `en` (지원 카탈로그가 없거나 환경변수 미설정 시)

---

## 4. API 및 키 네이밍 규칙

### 4.1 공개 API
```go
package i18n

func Init()
func SetLang(lang string)
func Lang() string
func T(key string, args ...any) string
func RelativeTime(t time.Time) string
```

- `T(key, args...)`: 플랫 도트(Dot) 키로 번역 템플릿을 조회하고 `args`가 있으면 `fmt.Sprintf`로 포맷합니다.
- `RelativeTime(t)`: 현재 `cmd/guard/main.go`와 `internal/adapters/tui/view.go`에 중복 구현된 상대 시간 포맷(`방금 전`/`just now`, `%d분 전`/`%dm ago`, `%d시간 전`/`%dh ago`)을 단일 함수로 통합합니다.

### 4.2 네임스페이스 규칙
| 네임스페이스 | 적용 대상 파일 | 주요 항목 |
| :--- | :--- | :--- |
| `cli.*` | `cmd/guard/main.go` | 도움말, 사용법 오류, `status`, `clean`, `reset`, `switchMode` 출력 |
| `launcher.*` | `internal/adapters/launcher/banner.go`, `launcher.go` | 시작 배너, 모드별 차단/허용 설명, 부트스트랩 알림, 실행 오류 |
| `tui.*` | `internal/adapters/tui/{view,modal,update,model}.go` | 헤더, 테이블 컬럼명, 이벤트 로그, 토스트, 도움말 오버레이, 모달 |
| `time.*` | `internal/i18n/i18n.go` | 상대 시간 표현 |
| `policy.builtin.*` | `internal/hub/policy/model.go` | 기본 내장 폴백 정책 위반 메시지 |

---

## 5. 번역 대상 vs 번역 제외 범위

| 구분 | 대상 | 비고 |
| :--- | :--- | :--- |
| **번역 대상** | CLI 출력, TUI UI/토스트/도움말/모달, 런처 배너, 내장 폴백 정책 메시지 | 사용자 대면 텍스트 전체 |
| **번역 제외** | 모드 식별자(`DEV`, `TEST`, `PLAN`, `ALL`), 훅 JSON 프로토콜 키(`decision`, `allow`, `deny`, `reason`), 사용자 작성 `guard-policy.yaml`의 `message` | 프로토콜/설정 불변값 |
| **선택 사항** | `guard doc` 출력 (AI Few-Shot 정책 스펙) | 현재 영문으로 작성되어 있어 영문 유지 권장 |

---

## 6. TUI 다국어 레이아웃 및 품질 검증

### 6.1 CJK / 영문 컬럼 폭 정렬
- `internal/adapters/tui/view.go`의 `padRight(s, width)`는 `lipgloss.Width`로 실제 터미널 표시 폭을 계산합니다.
- 각 테이블 컬럼의 고정 폭은 지원 언어(`ko`, `en`) 중 더 긴 라벨 기준으로 설정해 언어 전환 시에도 줄바꿈이나 열 밀림이 발생하지 않도록 유지합니다.
  - 예: `최근 활동`(9칸) / `Last Active`(11칸) → 고정 폭 `12칸`
  - 예: `모드`(4칸) / `Mode`(4칸) / `[TEST]`(6칸) → 고정 폭 `6칸`

### 6.2 자동 검증 테스트 (`internal/i18n/i18n_test.go`)
1. **키 정합성 테스트**: `en.json`과 `ko.json`의 키 집합이 100% 일치하는지 검증 (누락 키 방지).
2. **포맷 동사 검증**: 동일 키에 대해 `%s`, `%d`, `%v` 등 포맷 지정자 개수가 일치하는지 검증 (런타임 포맷 오류 방지).
3. **로케일 파싱 테스트**: `GUARD_LANG`, `LC_ALL`, `LANG` 조합(`ko_KR.UTF-8`, `en_US.UTF-8`, `C`, 미지원 언어) 검증.

---

## 7. 단계별 구현 계획

1. **Phase 1**: `internal/i18n` 패키지, `locales/en.json`·`locales/ko.json` 카탈로그 및 정합성 단위 테스트 구현
2. **Phase 2**: TUI(`internal/adapters/tui/`) 전체 문자열을 `i18n.T` 및 `i18n.RelativeTime`으로 전환
3. **Phase 3**: CLI(`cmd/guard/main.go`) 및 Launcher(`internal/adapters/launcher/`) 문자열 전환
4. **Phase 4**: 내장 폴백 정책(`internal/hub/policy/model.go`) 위반 메시지 전환 및 전체 `go test ./...` 검증
