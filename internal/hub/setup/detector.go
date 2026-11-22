package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"guard/internal/core"
)

const (
	PresetAuto    = "auto"
	PresetGo      = "go"
	PresetNode    = "node"
	PresetPython  = "python"
	PresetGeneric = "generic"
)

var (
	ErrInvalidPreset    = errors.New("invalid preset")
	ErrInvalidMode      = errors.New("invalid mode")
	ErrInvalidHooksJSON = errors.New("invalid hooks.json")
)

type stackMarker struct {
	file   string
	preset string
}

var stackMarkers = []stackMarker{
	{file: "go.mod", preset: PresetGo},
	{file: "package.json", preset: PresetNode},
	{file: "pyproject.toml", preset: PresetPython},
	{file: "requirements.txt", preset: PresetPython},
}

// DetectStack inspects project root marker files in priority order and returns
// the matching policy preset and marker filename. Falls back to ("generic", "").
func DetectStack(workspaceRoot string) (preset string, marker string) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return PresetGeneric, ""
	}
	for _, m := range stackMarkers {
		p := filepath.Join(workspaceRoot, m.file)
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			return m.preset, m.file
		}
	}
	return PresetGeneric, ""
}

// ValidatePreset checks whether preset is empty or one of the supported values
// (auto, go, node, python, generic), case-insensitively.
func ValidatePreset(preset string) error {
	p := strings.ToLower(strings.TrimSpace(preset))
	switch p {
	case "", PresetAuto, PresetGo, PresetNode, PresetPython, PresetGeneric:
		return nil
	default:
		return fmt.Errorf("%w: %q (supported: auto, go, node, python, generic)", ErrInvalidPreset, preset)
	}
}

func normalizeInitMode(mode string) (string, error) {
	raw := strings.TrimSpace(mode)
	if raw == "" {
		return core.ModeDev, nil
	}
	upper := strings.ToUpper(raw)
	switch upper {
	case core.ModeDev, core.ModePlan, core.ModeTest, core.ModeAll:
		return upper, nil
	default:
		return "", fmt.Errorf("%w: %q (supported: DEV, PLAN, TEST, ALL)", ErrInvalidMode, mode)
	}
}
