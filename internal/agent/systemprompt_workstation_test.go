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
	if !strings.Contains(prompt, "always use its argv array") {
		t.Fatal("system prompt is missing argv guidance for workstation execution")
	}
}
