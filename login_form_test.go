package rutracker

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/cookiejar"
	"strings"
	"testing"
)

// F64: the login form is sent as a browser on rutracker's cp1251 page
// sends it: "login" percent-encoded once, credentials in cp1251.
func TestLoginFormEncoding(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		http.SetCookie(w, &http.Cookie{Name: "bb_session", Value: "x"})
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	_ = authenticate(&http.Client{Jar: jar}, srv.URL+"/", "Иван", "пароль1")

	for _, want := range []string{
		"login=%C2%F5%EE%E4",                  // "Вход", once
		"login_username=%C8%E2%E0%ED",         // "Иван" in cp1251
		"login_password=%EF%E0%F0%EE%EB%FC1", // "пароль1" in cp1251
	} {
		if !strings.Contains(body, want) {
			t.Errorf("form body %q lacks %q", body, want)
		}
	}
	if strings.Contains(body, "%25") {
		t.Errorf("form body %q is double-encoded", body)
	}
}

func TestToCP1251KeepsASCIIAndUnencodable(t *testing.T) {
	if got := toCP1251("user_1"); got != "user_1" {
		t.Errorf("ASCII changed: %q", got)
	}
	if got := toCP1251("emoji🙂"); got != "emoji🙂" {
		t.Errorf("unencodable input changed: %q", got)
	}
}
