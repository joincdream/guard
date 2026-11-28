# Guard Black-Box Test Contracts (`doc/contract/`)

이 디렉토리는 **TEST 모드(Cleanroom)** 에서 프로덕션 소스 코드를 열람하지 않고도 블랙박스 테스트(`package ..._test`)를 작성할 수 있도록 **공개 API 시그니처, DTO 구조, 에러 규약, CLI/TUI 동작 계약**을 정의한다.

## 문서 목록
1. [01_custom_mode_contract.md](01_custom_mode_contract.md) — 동적(커스텀) 모드 추출·검증·핸드오프·CLI 라우팅 계약 (`internal/core`, `internal/hub/handoff`, `cmd/guard`)
2. [02_tui_handoff_ux_contract.md](02_tui_handoff_ux_contract.md) — TUI 핸드오프 모달(`Payload`, `Esc` 보류 vs `X`/`C` 반려, 다중 참조 전환) 및 모드 순환(`m`/`M`) 계약 (`internal/adapters/tui`)

## 테스트 작성 원칙
* 테스트 파일은 `package <pkg>_test` (외부 블랙박스 테스트)로 작성한다.
* 아래 계약에 명시된 공개 심볼과 동작 규격만을 기준으로 단언(Assert)한다.
