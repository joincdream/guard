package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type StepStatus string

const (
	StatusCreated  StepStatus = "created"
	StatusMerged   StepStatus = "merged"
	StatusSkipped  StepStatus = "skipped"
	StatusPlanned  StepStatus = "planned"
	StatusBackedUp StepStatus = "backed_up"
)

type HookMergeResult struct {
	Path          string
	Status        StepStatus
	PreservedKeys []string
}

func standardGuardPreToolUseEntry() map[string]any {
	return map[string]any{
		"matcher": "*",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": "guard hook",
			},
		},
	}
}

func standardGuardHookBlock() map[string]any {
	return map[string]any{
		"PreToolUse": []any{
			standardGuardPreToolUseEntry(),
		},
	}
}

// InspectWorkspaceHook inspects <workspaceRoot>/.agents/hooks.json without modifying it.
func InspectWorkspaceHook(workspaceRoot string) (exists bool, validJSON bool, hasGuardHook bool, err error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return false, false, false, errors.New("workspace root is required")
	}
	hookPath := filepath.Join(workspaceRoot, ".agents", "hooks.json")
	data, readErr := os.ReadFile(hookPath)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return false, false, false, nil
		}
		return false, false, false, readErr
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return true, false, false, nil
	}

	var root map[string]any
	if unmarshalErr := json.Unmarshal(data, &root); unmarshalErr != nil || root == nil {
		return true, false, false, nil
	}

	return true, true, hasGuardPreToolUseHook(root), nil
}

// MergeWorkspaceHook safely registers the guard PreToolUse hook inside
// <workspaceRoot>/.agents/hooks.json while preserving all other top-level keys.
// Invalid JSON files are never modified and return an error wrapping ErrInvalidHooksJSON.
func MergeWorkspaceHook(workspaceRoot string, dryRun bool) (HookMergeResult, error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return HookMergeResult{}, errors.New("workspace root is required")
	}
	hookPath := filepath.Join(workspaceRoot, ".agents", "hooks.json")
	res := HookMergeResult{Path: hookPath}

	root := make(map[string]any)
	fileExists := false

	data, readErr := os.ReadFile(hookPath)
	if readErr != nil {
		if !errors.Is(readErr, os.ErrNotExist) {
			return res, fmt.Errorf("read %s: %w", hookPath, readErr)
		}
	} else {
		fileExists = true
		if len(strings.TrimSpace(string(data))) == 0 {
			return res, fmt.Errorf("%w: %s is empty", ErrInvalidHooksJSON, hookPath)
		}
		if unmarshalErr := json.Unmarshal(data, &root); unmarshalErr != nil || root == nil {
			return res, fmt.Errorf("%w: %v", ErrInvalidHooksJSON, unmarshalErr)
		}
		for k := range root {
			if k != "guard" {
				res.PreservedKeys = append(res.PreservedKeys, k)
			}
		}
		sort.Strings(res.PreservedKeys)
	}

	if fileExists && hasGuardPreToolUseHook(root) {
		res.Status = StatusSkipped
		return res, nil
	}

	if dryRun {
		res.Status = StatusPlanned
		return res, nil
	}

	if existingGuard, exists := root["guard"]; !exists || existingGuard == nil {
		root["guard"] = standardGuardHookBlock()
	} else {
		guardMap, ok := existingGuard.(map[string]any)
		if !ok {
			return res, fmt.Errorf("%w: top-level 'guard' entry is not a JSON object", ErrInvalidHooksJSON)
		}
		rawPre, hasPre := guardMap["PreToolUse"]
		if !hasPre || rawPre == nil {
			guardMap["PreToolUse"] = []any{standardGuardPreToolUseEntry()}
		} else {
			preList, ok := rawPre.([]any)
			if !ok {
				return res, fmt.Errorf("%w: 'guard.PreToolUse' is not a JSON array", ErrInvalidHooksJSON)
			}
			guardMap["PreToolUse"] = append(preList, standardGuardPreToolUseEntry())
		}
		root["guard"] = guardMap
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return res, fmt.Errorf("marshal hooks.json: %w", err)
	}
	out = append(out, '\n')

	if err := writeFileAtomic(hookPath, out, 0o644); err != nil {
		return res, err
	}

	if fileExists {
		res.Status = StatusMerged
	} else {
		res.Status = StatusCreated
	}
	return res, nil
}

func hasGuardPreToolUseHook(root map[string]any) bool {
	rawGuard, exists := root["guard"]
	if !exists || rawGuard == nil {
		return false
	}
	guardMap, ok := rawGuard.(map[string]any)
	if !ok {
		return false
	}
	rawPre, exists := guardMap["PreToolUse"]
	if !exists || rawPre == nil {
		return false
	}
	preList, ok := rawPre.([]any)
	if !ok {
		return false
	}
	for _, entry := range preList {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		rawHooks, ok := entryMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range rawHooks {
			hMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hMap["command"].(string)
			cmd = strings.TrimSpace(cmd)
			if cmd == "guard hook" || cmd == "guard" || strings.HasSuffix(cmd, "/guard hook") {
				return true
			}
		}
	}
	return false
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("open temp file %s: %w", tmpPath, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s to %s: %w", tmpPath, path, err)
	}
	return nil
}
