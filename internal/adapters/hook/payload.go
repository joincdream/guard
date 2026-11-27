package hook

import "strings"

// Input represents the Antigravity CLI PreToolUse JSON payload passed via stdin.
type Input struct {
	SessionID      string   `json:"sessionId"`
	SessionIDSnake string   `json:"session_id"`
	ConversationID string   `json:"conversationId"`
	CWD            string   `json:"cwd"`
	WorkspacePaths []string `json:"workspacePaths"`
	ToolCall       struct {
		ID   string                 `json:"id"`
		Name string                 `json:"name"`
		Args map[string]interface{} `json:"args"`
	} `json:"toolCall"`
	ToolCallID  string                 `json:"tool_call_id"`
	ToolName    string                 `json:"tool_name"`
	ToolArgs    map[string]interface{} `json:"tool_args"`
	ToolInput   map[string]interface{} `json:"tool_input"`
	Path        string                 `json:"Path"`
	PathLower   string                 `json:"path"`
	CommandLine string                 `json:"CommandLine"`
	CmdLower    string                 `json:"command_line"`
}

// Output represents the JSON decision returned to Antigravity CLI via stdout.
type Output struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// ExtractSessionID returns the first non-empty session identifier from the payload.
func (in *Input) ExtractSessionID() string {
	for _, v := range []string{in.SessionID, in.SessionIDSnake, in.ConversationID} {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// ExtractToolCallID returns the unique tool invocation identifier from the payload if present.
func (in *Input) ExtractToolCallID() string {
	for _, v := range []string{in.ToolCall.ID, in.ToolCallID} {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// ExtractToolName returns the invoked tool name from supported payload shapes.
func (in *Input) ExtractToolName() string {
	if s := strings.TrimSpace(in.ToolCall.Name); s != "" {
		return s
	}
	return strings.TrimSpace(in.ToolName)
}

// ExtractTargetPath returns the file path argument from supported payload shapes.
func (in *Input) ExtractTargetPath() string {
	if p := stringArg(in.ToolCall.Args, "Path", "path", "file_path"); p != "" {
		return p
	}
	if p := stringArg(in.ToolArgs, "Path", "path", "file_path"); p != "" {
		return p
	}
	if p := stringArg(in.ToolInput, "Path", "path", "file_path"); p != "" {
		return p
	}
	if strings.TrimSpace(in.Path) != "" {
		return strings.TrimSpace(in.Path)
	}
	return strings.TrimSpace(in.PathLower)
}

// ExtractCommandLine returns the shell command line argument from supported payload shapes.
func (in *Input) ExtractCommandLine() string {
	if c := stringArg(in.ToolCall.Args, "CommandLine", "command_line", "command", "cmd"); c != "" {
		return c
	}
	if c := stringArg(in.ToolArgs, "CommandLine", "command_line", "command", "cmd"); c != "" {
		return c
	}
	if c := stringArg(in.ToolInput, "CommandLine", "command_line", "command", "cmd"); c != "" {
		return c
	}
	if strings.TrimSpace(in.CommandLine) != "" {
		return strings.TrimSpace(in.CommandLine)
	}
	return strings.TrimSpace(in.CmdLower)
}

// ExtractCWD returns the agent working directory from the payload if present.
func (in *Input) ExtractCWD() string {
	if s := strings.TrimSpace(in.CWD); s != "" {
		return s
	}
	if len(in.WorkspacePaths) > 0 {
		return strings.TrimSpace(in.WorkspacePaths[0])
	}
	return ""
}

func stringArg(args map[string]interface{}, keys ...string) string {
	if len(args) == 0 {
		return ""
	}
	for _, k := range keys {
		if v, ok := args[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
