package agent

import (
	"strings"
	"testing"
)

func TestSystemPromptGuidesWorkstationCallsToArgv(t *testing.T) {
	prompt := BuildSystemPrompt(SystemPromptConfig{
		Mode:      PromptFull,
		ToolNames: []string{"workstation_exec"},
	})
	if !strings.Contains(prompt, "always use its argv array") || !strings.Contains(prompt, "pass the script through stdin") {
		t.Fatal("system prompt is missing argv/stdin guidance for workstation execution")
	}
}
