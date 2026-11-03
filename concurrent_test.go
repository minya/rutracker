package rutracker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/minya/goutils/web"
)

// F20: one client is shared by every request of a process, and any of them
// may log in again when the session expires. Run with -race: logging in
// must not modify the shared http.Client.
func TestClientIsSafeForConcurrentUse(t *testing.T) {
	f := &fakeRutracker{search: `<table></table>`}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	c := &RutrackerClient{username: "u", password: "p", baseURL: srv.URL + "/forum/",
		httpClient: &http.Client{Jar: web.NewJar()}}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := c.FindContext(context.Background(), "x"); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := authenticate(context.Background(), c.httpClient, c.baseURL, "u", "p"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if c.httpClient.CheckRedirect != nil {
		t.Error("authenticate left a redirect policy on the shared client")
	}
}
