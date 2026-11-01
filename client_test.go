package rutracker

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/minya/goutils/web"
)

const loginPage = `<html><form action="login.php"><input name="login_username"><input name="login_password"></form></html>`

// fakeRutracker serves login.php, tracker.php and dl.php. Requests carry a
// session only after a login; expire drops the session once.
type fakeRutracker struct {
	logins   atomic.Int32
	loggedIn atomic.Bool
	search   string
	download func(w http.ResponseWriter)
}

func (f *fakeRutracker) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/login.php"):
			f.logins.Add(1)
			f.loggedIn.Store(true)
			http.SetCookie(w, &http.Cookie{Name: "bb_session", Value: "x"})
			w.Header().Set("Location", "index.php")
			w.WriteHeader(http.StatusFound)
		case !f.loggedIn.Load():
			w.Write([]byte(loginPage))
		case strings.HasSuffix(r.URL.Path, "/tracker.php"):
			w.Write([]byte(f.search))
		case strings.HasSuffix(r.URL.Path, "/dl.php"):
			f.download(w)
		default:
			http.NotFound(w, r)
		}
	})
}

func newTestClient(t *testing.T, f *fakeRutracker) *RutrackerClient {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return &RutrackerClient{
		username:   "u",
		password:   "p",
		httpClient: &http.Client{Jar: web.NewJar()},
		baseURL:    srv.URL + "/forum/",
	}
}

func TestDownloadTorrentReturnsTorrent(t *testing.T) {
	f := &fakeRutracker{download: func(w http.ResponseWriter) { w.Write([]byte("d8:announce3:urle")) }}
	f.loggedIn.Store(true)
	body, err := newTestClient(t, f).DownloadTorrent("dl.php?t=1")
	if err != nil || string(body) != "d8:announce3:urle" {
		t.Fatalf("DownloadTorrent = %q, %v", body, err)
	}
}

func TestDownloadTorrentRejectsErrorStatus(t *testing.T) {
	f := &fakeRutracker{download: func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("down"))
	}}
	f.loggedIn.Store(true)
	_, err := newTestClient(t, f).DownloadTorrent("dl.php?t=1")
	var se *StatusError
	if !errors.As(err, &se) || se.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want StatusError 503", err)
	}
}

func TestDownloadTorrentRejectsHTML(t *testing.T) {
	f := &fakeRutracker{download: func(w http.ResponseWriter) { w.Write([]byte("<html>topic not found</html>")) }}
	f.loggedIn.Store(true)
	if _, err := newTestClient(t, f).DownloadTorrent("dl.php?t=1"); !errors.Is(err, ErrNotTorrent) {
		t.Fatalf("err = %v, want ErrNotTorrent", err)
	}
}

func TestDownloadTorrentRejectsOversizedBody(t *testing.T) {
	f := &fakeRutracker{download: func(w http.ResponseWriter) {
		w.Write([]byte("d"))
		w.Write(make([]byte, maxTorrentSize))
	}}
	f.loggedIn.Store(true)
	if _, err := newTestClient(t, f).DownloadTorrent("dl.php?t=1"); err == nil {
		t.Fatal("oversized body was accepted")
	}
}

// An expired session is restored by one new login, then the request is retried.
func TestExpiredSessionLogsInAgain(t *testing.T) {
	f := &fakeRutracker{download: func(w http.ResponseWriter) { w.Write([]byte("de")) }}
	c := newTestClient(t, f) // not logged in: the first request gets the login page
	body, err := c.DownloadTorrent("dl.php?t=1")
	if err != nil || string(body) != "de" {
		t.Fatalf("DownloadTorrent = %q, %v", body, err)
	}
	if n := f.logins.Load(); n != 1 {
		t.Fatalf("logged in %d times, want 1", n)
	}
}

// When logging in does not help, the error says so instead of returning the
// login page as a result.
func TestLoginPageAfterReloginIsNotLoggedIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/login.php") {
			w.WriteHeader(http.StatusFound)
			return
		}
		w.Write([]byte(loginPage))
	}))
	defer srv.Close()
	c := &RutrackerClient{httpClient: &http.Client{Jar: web.NewJar()}, baseURL: srv.URL + "/forum/"}

	if _, err := c.Find("query"); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("Find err = %v, want ErrNotLoggedIn", err)
	}
	if _, err := c.DownloadTorrent("dl.php?t=1"); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("DownloadTorrent err = %v, want ErrNotLoggedIn", err)
	}
}

func TestFindRejectsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := &RutrackerClient{httpClient: &http.Client{Jar: web.NewJar()}, baseURL: srv.URL + "/forum/"}
	var se *StatusError
	if _, err := c.Find("q"); !errors.As(err, &se) || se.StatusCode != http.StatusBadGateway {
		t.Fatalf("err = %v, want StatusError 502", err)
	}
}

// An ordinary empty result is still an empty list, not an error.
func TestFindEmptyResultIsNotAnError(t *testing.T) {
	f := &fakeRutracker{search: "<html><table id=\"tor-tbl\"></table></html>"}
	f.loggedIn.Store(true)
	items, err := newTestClient(t, f).Find("nothing")
	if err != nil || len(items) != 0 {
		t.Fatalf("Find = %v, %v", items, err)
	}
}
