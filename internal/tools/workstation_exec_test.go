package tools

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type blockingWorkstationStream struct {
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
	stderrR *io.PipeReader
	stderrW *io.PipeWriter
	killN   atomic.Int64
	once    sync.Once
	done    chan struct{}
}

func newBlockingWorkstationStream() *blockingWorkstationStream {
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	return &blockingWorkstationStream{
		stdoutR: stdoutR,
		stdoutW: stdoutW,
		stderrR: stderrR,
		stderrW: stderrW,
		done:    make(chan struct{}),
	}
}

func (s *blockingWorkstationStream) Stdout() io.Reader { return s.stdoutR }

func (s *blockingWorkstationStream) Stderr() io.Reader { return s.stderrR }

func (s *blockingWorkstationStream) Wait() (int, error) {
	<-s.done
	return 137, errors.New("killed")
}

func (s *blockingWorkstationStream) Kill() error {
	s.killN.Add(1)
	s.once.Do(func() {
		_ = s.stdoutW.CloseWithError(context.Canceled)
		_ = s.stderrW.CloseWithError(context.Canceled)
		close(s.done)
	})
	return nil
}

func TestStreamAndCollectTimeoutKillsBlockedReaders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	stream := newBlockingWorkstationStream()
	tool := &WorkstationExecTool{}
	ws := &store.Workstation{
		ID:       uuid.New(),
		TenantID: uuid.New(),
	}

	done := make(chan *Result, 1)
	go func() {
		done <- tool.streamAndCollect(ctx, stream, ws, uuid.NewString(), "session-timeout", "sleep 60")
	}()

	select {
	case result := <-done:
		if !result.IsError {
			t.Fatalf("expected timeout result to be an error, got %#v", result)
		}
		if stream.killN.Load() == 0 {
			t.Fatal("expected timed-out stream to be killed")
		}
	case <-time.After(time.Second):
		t.Fatal("streamAndCollect did not return after context timeout")
	}
}

func TestParseWorkstationInvocationPrefersArgv(t *testing.T) {
	cmd, args, err := parseWorkstationInvocation(map[string]any{
		"argv":    []any{"curl", "-fsS", "https://example.test/health"},
		"command": "",
		"args":    []any{},
	})
	if err != nil {
		t.Fatalf("parseWorkstationInvocation() error = %v", err)
	}
	if cmd != "curl" {
		t.Errorf("command = %q, want curl", cmd)
	}
	want := []string{"-fsS", "https://example.test/health"}
	if len(args) != len(want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestParseWorkstationInvocationRetainsLegacyCommandArgs(t *testing.T) {
	cmd, args, err := parseWorkstationInvocation(map[string]any{
		"command": "git",
		"args":    []any{"status", "--short"},
	})
	if err != nil {
		t.Fatalf("parseWorkstationInvocation() error = %v", err)
	}
	if cmd != "git" {
		t.Errorf("command = %q, want git", cmd)
	}
	if len(args) != 2 || args[0] != "status" || args[1] != "--short" {
		t.Errorf("args = %#v, want [status --short]", args)
	}
}

func TestParseWorkstationInvocationAllowsLongArgWithinLimit(t *testing.T) {
	// The production failure was a 3,285-byte script/body passed as argv[2].
	// Keep coverage at the full configured limit so future changes cannot silently
	// restore the historical 1 KiB cap.
	longArg := strings.Repeat("x", execMaxArgBytes)
	cmd, args, err := parseWorkstationInvocation(map[string]any{
		"argv": []any{"sh", "-c", longArg},
	})
	if err != nil {
		t.Fatalf("parseWorkstationInvocation() error = %v", err)
	}
	if cmd != "sh" || len(args) != 2 || args[1] != longArg {
		t.Fatalf("invocation = %q %#v, want long argv item preserved", cmd, args)
	}

	_, _, err = parseWorkstationInvocation(map[string]any{
		"argv": []any{"sh", "-c", longArg + "x"},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds 16384 byte limit") {
		t.Fatalf("oversized argv error = %v, want 16 KiB limit", err)
	}
}

func TestParseWorkstationInvocationRejectsAmbiguousOrControlInput(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
		want  string
	}{
		{
			name:  "both invocation forms",
			input: map[string]any{"argv": []any{"git", "status"}, "command": "git"},
			want:  "either argv or command + args",
		},
		{
			name:  "newline in executable",
			input: map[string]any{"argv": []any{"curl\n-s"}},
			want:  "invalid newline",
		},
		{
			name:  "nul in legacy command",
			input: map[string]any{"command": "curl\x00-s"},
			want:  "invalid NUL byte",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseWorkstationInvocation(tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseWorkstationInvocation() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestWorkstationExecSchemaOffersArgvAndLegacyCompatibility(t *testing.T) {
	params := (&WorkstationExecTool{}).Parameters()
	properties, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties missing from workstation exec schema")
	}
	argv, ok := properties["argv"].(map[string]any)
	if !ok || argv["minItems"] != 1 {
		t.Fatalf("argv schema = %#v, want non-empty argv array", argv)
	}
	if _, ok := properties["command"]; !ok {
		t.Fatal("legacy command field missing")
	}
	if required, ok := params["required"].([]string); !ok || len(required) != 1 || required[0] != "argv" {
		t.Fatalf("required = %#v, want argv", params["required"])
	}
}

func TestWorkstationExecSchemaKeepsArgvContractForOpenAICompat(t *testing.T) {
	defs := providers.CleanToolSchemas("openai_compat", []providers.ToolDefinition{
		ToProviderDef(&WorkstationExecTool{}),
	})
	if len(defs) != 1 || defs[0].Function == nil {
		t.Fatalf("cleaned definitions = %#v, want one function definition", defs)
	}
	params := defs[0].Function.Parameters
	properties, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("cleaned properties missing")
	}
	if _, ok := properties["argv"]; !ok {
		t.Fatal("openai_compat schema lost the argv property")
	}
	if required, ok := params["required"].([]string); !ok || len(required) != 1 || required[0] != "argv" {
		t.Fatalf("cleaned required = %#v, want argv", params["required"])
	}
}
