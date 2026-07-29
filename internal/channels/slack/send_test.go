package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	slackapi "github.com/slack-go/slack"
)

func TestSend_UploadsMediaAfterUpdatingPlaceholder(t *testing.T) {
	ch, calls := newUploadTestChannel(t)
	ch.placeholders.Store("C123", "1.0")

	err := ch.Send(context.Background(), bus.OutboundMessage{
		ChatID:  "C123",
		Content: "report ready",
		Media:   []bus.MediaAttachment{newUploadTestAttachment(t)},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	assertUploadCalls(t, calls, 1)
	if got := calls("/chat.update"); got != 1 {
		t.Errorf("requests to /chat.update = %d, want 1", got)
	}
}

func TestSend_UploadsMediaWithoutText(t *testing.T) {
	ch, calls := newUploadTestChannel(t)
	ch.placeholders.Store("C123", "1.0")

	err := ch.Send(context.Background(), bus.OutboundMessage{
		ChatID: "C123",
		Media:  []bus.MediaAttachment{newUploadTestAttachment(t)},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	assertUploadCalls(t, calls, 1)
	if got := calls("/chat.update"); got != 0 {
		t.Errorf("requests to /chat.update = %d, want 0", got)
	}
	if got := calls("/chat.delete"); got != 1 {
		t.Errorf("requests to /chat.delete = %d, want 1", got)
	}
}

func newUploadTestChannel(t *testing.T) (*Channel, func(string) int) {
	t.Helper()

	var mu sync.Mutex
	calls := make(map[string]int)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/chat.update", "/chat.delete":
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1.0","message":{"text":"report ready"}}`))
		case "/files.getUploadURLExternal":
			_, _ = w.Write([]byte(`{"ok":true,"upload_url":"` + server.URL + `/upload","file_id":"F123"}`))
		case "/upload":
			w.WriteHeader(http.StatusOK)
		case "/files.completeUploadExternal":
			_, _ = w.Write([]byte(`{"ok":true,"files":[{"id":"F123","title":"report.txt"}]}`))
		default:
			t.Errorf("unexpected Slack API request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	ch := &Channel{
		BaseChannel: channels.NewBaseChannel(channels.TypeSlack, nil, nil),
		api:         slackapi.New("xoxb-test", slackapi.OptionAPIURL(server.URL+"/")),
	}
	ch.SetRunning(true)

	return ch, func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return calls[path]
	}
}

func newUploadTestAttachment(t *testing.T) bus.MediaAttachment {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(filePath, []byte("report"), 0o600); err != nil {
		t.Fatal(err)
	}
	return bus.MediaAttachment{URL: filePath, ContentType: "text/plain"}
}

func assertUploadCalls(t *testing.T, calls func(string) int, want int) {
	t.Helper()

	for _, path := range []string{"/files.getUploadURLExternal", "/upload", "/files.completeUploadExternal"} {
		if got := calls(path); got != want {
			t.Errorf("requests to %s = %d, want %d", path, got, want)
		}
	}
}
