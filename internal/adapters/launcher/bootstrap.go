package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const guardSkillContent = `---
name: guard
description: Antigravity CLI 가드레일(guard) 정책 파일(guard-policy.yaml)을 프로젝트 구조에 맞게 자동 생성·수정·검증할 때 사용하는 스킬. 사용자가 'guard 정책 만들어줘', '가드레일 규칙 추가', 'guard-policy.yaml 수정' 등을 요청할 때 활성화하십시오.
---

# Guard Policy Authoring Skill

사용자가 가드레일 정책(` + "`guard-policy.yaml`" + `) 생성 또는 수정을 요청하면 다음 절차를 따릅니다.

## 절차
1. **최신 스키마 및 Few-Shot 샘플 조회**:
   먼저 터미널에서 ` + "`guard doc`" + ` 명령을 실행해 최신 정책 스키마, 패턴 문법, 탐색 우선순위 및 완성형 샘플을 확인합니다.
2. **프로젝트 구조 파악**:
   현재 워크스페이스의 언어/디렉터리 구조(소스 경로, 테스트 파일 패턴, 문서 경로, 민감 파일)를 확인합니다.
3. **정책 파일 작성/수정**:
   ` + "`./guard-policy.yaml`" + ` (또는 요청된 경로)에 ` + "`DEV`" + `, ` + "`PLAN`" + `, ` + "`TEST`" + ` 및 전역 규칙을 작성합니다.
4. **즉시 검증**:
   - ` + "`guard status`" + `로 작성한 정책 파일이 정상 로드되었는지 확인합니다.
   - 대표적인 허용/차단 케이스를 ` + "`echo '{...}' | AGENT_MODE=DEV guard`" + ` 파이프로 실행해 기대한 ` + "`allow`" + `/` + "`deny`" + ` 결과가 나오는지 검증합니다.
`

// EnsureAgentSkill installs ~/.gemini/antigravity-cli/skills/guard/SKILL.md if it does not yet exist.
// Existing files are left untouched so user customizations are preserved.
func EnsureAgentSkill(homeDir string) (skillPath string, created bool, err error) {
	if strings.TrimSpace(homeDir) == "" {
		return "", false, nil
	}
	skillPath = filepath.Join(homeDir, ".gemini", "antigravity-cli", "skills", "guard", "SKILL.md")
	if _, statErr := os.Stat(skillPath); statErr == nil {
		return skillPath, false, nil
	}

	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		return skillPath, false, err
	}
	if err := os.WriteFile(skillPath, []byte(guardSkillContent), 0o644); err != nil {
		return skillPath, false, err
	}
	return skillPath, true, nil
}

// HasWorkspaceHook reports whether <workspace>/.agents/hooks.json exists and references guard.
func HasWorkspaceHook(workspace string) bool {
	if strings.TrimSpace(workspace) == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(workspace, ".agents", "hooks.json"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "guard")
}

// EnsureWorkspaceHook registers the guard PreToolUse hook inside <workspace>/.agents/hooks.json
// while preserving any existing hook entries (such as chatlog).
func EnsureWorkspaceHook(workspace string) (hookPath string, updated bool, err error) {
	if strings.TrimSpace(workspace) == "" {
		return "", false, nil
	}
	hookPath = filepath.Join(workspace, ".agents", "hooks.json")

	root := make(map[string]interface{})
	if data, readErr := os.ReadFile(hookPath); readErr == nil && len(strings.TrimSpace(string(data))) > 0 {
		if unmarshalErr := json.Unmarshal(data, &root); unmarshalErr != nil {
			// Do not corrupt an unparseable hooks.json file.
			return hookPath, false, unmarshalErr
		}
		if _, exists := root["guard"]; exists {
			return hookPath, false, nil
		}
	}

	root["guard"] = map[string]interface{}{
		"PreToolUse": []interface{}{
			map[string]interface{}{
				"matcher": "*",
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": "guard hook",
					},
				},
			},
		},
	}

	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return hookPath, false, err
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return hookPath, false, err
	}
	out = append(out, '\n')
	if err := os.WriteFile(hookPath, out, 0o644); err != nil {
		return hookPath, false, err
	}
	return hookPath, true, nil
}
