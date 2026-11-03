package rutracker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/minya/goutils/web"
)

// F65: a cancelled or timed-out context aborts login, search and download
// instead of waiting for the client timeout.
func TestContextAbortsRequests(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // a rutracker that never answers
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer srv.Close()
	defer close(done)
	c := &RutrackerClient{username: "u", password: "p", baseURL: srv.URL + "/forum/",
		httpClient: &http.Client{Jar: web.NewJar(), Timeout: 30 * time.Second}}

	for name, call := range map[string]func(context.Context) error{
		"find":     func(ctx context.Context) error { _, err := c.FindContext(ctx, "x"); return err },
		"download": func(ctx context.Context) error { _, err := c.DownloadTorrentContext(ctx, "dl.php?t=1"); return err },
		"login":    func(ctx context.Context) error { return authenticate(ctx, c.httpClient, c.baseURL, "u", "p") },
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		start := time.Now()
		err := call(ctx)
		cancel()
		if took := time.Since(start); took > 2*time.Second {
			t.Errorf("%s took %s after a 200 ms deadline", name, took)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%s: err = %v, want context.DeadlineExceeded", name, err)
		}
	}
}

func TestFailedLoginReturnsZeroClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c, err := NewAuthenticatedRutrackerClientContext(ctx, "u", "p")
	if err == nil || c.httpClient != nil {
		t.Fatalf("client = %+v, err = %v; want the zero client and an error", c, err)
	}
}
