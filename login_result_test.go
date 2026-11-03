package rutracker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/cookiejar"
	"testing"
)

// F2: only a redirect that sets bb_session is a successful login. A wrong
// password or a captcha answers 200 with the login form again.
func TestAuthenticateDetectsFailedLogin(t *testing.T) {
	for _, tc := range []struct {
		name    string
		respond func(w http.ResponseWriter)
		want    error
	}{
		{"success", func(w http.ResponseWriter) {
			http.SetCookie(w, &http.Cookie{Name: "bb_session", Value: "0-123-abc"})
			w.Header().Set("Location", "index.php")
			w.WriteHeader(http.StatusFound)
		}, nil},
		{"wrong password", func(w http.ResponseWriter) {
			w.Write([]byte(`<html><h4 class="warnColor1">неверный пароль</h4>` + loginPage + `</html>`))
		}, ErrLoginFailed},
		{"captcha", func(w http.ResponseWriter) {
			w.Write([]byte(`<html><form><img src="//static.t-ru.org/captcha/x.jpg"><input type="hidden" name="cap_sid" value="s"></form>` + loginPage + `</html>`))
		}, ErrCaptcha},
		{"redirect without session", func(w http.ResponseWriter) {
			w.Header().Set("Location", "index.php")
			w.WriteHeader(http.StatusFound)
		}, ErrLoginFailed},
		{"server error", func(w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) }, ErrLoginFailed},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { tc.respond(w) }))
		jar, _ := cookiejar.New(nil)
		err := authenticate(context.Background(), &http.Client{Jar: jar}, srv.URL+"/forum/", "u", "p")
		srv.Close()
		if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}
