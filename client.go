package rutracker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minya/goutils/web"
	"github.com/minya/logger"
)

type RutrackerClient struct {
	username   string
	password   string
	httpClient *http.Client
	// baseURL is the forum root, "https://rutracker.org/forum/"; tests
	// point it at a fake server.
	baseURL string
}

const defaultBaseURL = "https://rutracker.org/forum/"

// maxTorrentSize caps how much DownloadTorrent reads; .torrent files are
// kilobytes to a few megabytes.
const maxTorrentSize = 10 << 20

var (
	// ErrNotLoggedIn is returned when rutracker answers with its login page
	// even after logging in again: the session could not be restored (wrong
	// credentials, captcha, or a changed login flow).
	ErrNotLoggedIn = errors.New("rutracker: not logged in")
	// ErrNotTorrent is returned when a download does not return a .torrent
	// file (e.g. an HTML error page).
	ErrNotTorrent = errors.New("rutracker: response is not a .torrent file")
)

// StatusError is returned for an HTTP status other than 200.
type StatusError struct {
	Op         string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("rutracker %s: unexpected HTTP status %d", e.Op, e.StatusCode)
}

type Option func(*options)

type options struct {
	timeout  time.Duration
	forceIP6 bool
}

var defaultOptions = options{
	timeout: 5 * time.Second,
}

// WithTimeout sets the HTTP client timeout for all requests.
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.timeout = d
	}
}

// WithIPv6 forces all connections to use IPv6.
func WithIPv6() Option {
	return func(o *options) {
		o.forceIP6 = true
	}
}

func NewAuthenticatedRutrackerClient(username string, password string, opts ...Option) (RutrackerClient, error) {
	o := defaultOptions
	for _, opt := range opts {
		opt(&o)
	}

	transport := web.DefaultTransport(5000)
	if o.forceIP6 {
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, "tcp6", addr)
		}
	}

	httpClient := http.Client{
		Jar:       web.NewJar(),
		Transport: transport,
		Timeout:   o.timeout,
	}

	client := RutrackerClient{
		username:   username,
		password:   password,
		httpClient: &httpClient,
		baseURL:    defaultBaseURL,
	}

	err := authenticate(&httpClient, client.baseURL, username, password)
	if err != nil {
		return client, err
	}
	return client, nil
}

func authenticate(httpClient *http.Client, baseURL, username string, password string) error {
	rutrackerLoginURL := baseURL + "login.php"
	form := url.Values{}
	form.Set("login_username", username)
	form.Set("login_password", password)
	form.Set("login", "%C2%F5%EE%E4")
	formData := strings.NewReader(form.Encode())

	// Disable redirect following — we only need the session cookie
	// from the login response, not the redirect target.
	origCheckRedirect := httpClient.CheckRedirect
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	res, err := httpClient.Post(rutrackerLoginURL, "application/x-www-form-urlencoded", formData)
	httpClient.CheckRedirect = origCheckRedirect
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 && res.StatusCode != http.StatusFound && res.StatusCode != http.StatusMovedPermanently {
		return fmt.Errorf("rutracker login failed. Status code: %v", res.StatusCode)
	}
	return nil
}

// isLoginPage reports whether body is rutracker's logged-out page: it then
// carries the login form, which logged-in pages do not.
func isLoginPage(body []byte) bool {
	return bytes.Contains(body, []byte(`name="login_username"`))
}

// fetch performs do and returns the body. When rutracker answers with its
// login page (the session expired), it logs in again once and retries; if
// the login page comes back again it returns ErrNotLoggedIn.
func (c *RutrackerClient) fetch(op string, limit int64, do func() (*http.Response, error)) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		res, err := do()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		if res.StatusCode != http.StatusOK {
			return nil, &StatusError{Op: op, StatusCode: res.StatusCode}
		}
		if int64(len(body)) > limit {
			return nil, fmt.Errorf("rutracker %s: response larger than %d bytes", op, limit)
		}
		if !isLoginPage(body) {
			return body, nil
		}
		if attempt > 0 {
			return nil, ErrNotLoggedIn
		}
		logger.Info("rutracker session expired; logging in again")
		if err := authenticate(c.httpClient, c.baseURL, c.username, c.password); err != nil {
			return nil, err
		}
	}
}

// Find searches rutracker. It returns an error for a non-200 response and
// ErrNotLoggedIn when the session cannot be restored, so "no results" is
// distinguishable from a failed search.
func (c *RutrackerClient) Find(pattern string) ([]RutrackerSearchItem, error) {
	searchURL := fmt.Sprintf("%stracker.php?nm=%s", c.baseURL, url.QueryEscape(pattern))
	searchBodyData := url.Values{}
	searchBodyData.Set("nm", pattern)
	responseBodyData, err := c.fetch("search", 32<<20, func() (*http.Response, error) {
		return c.httpClient.Post(searchURL, "application/x-www-form-urlencoded", strings.NewReader(searchBodyData.Encode()))
	})
	if err != nil {
		logger.Error(err, "Search failed")
		return nil, err
	}

	items, err := ParseSearchItems(&responseBodyData)
	if err != nil {
		logger.Error(err, "Failed to parse search items")
		return nil, err
	}

	return items, nil
}

// DownloadTorrent fetches a .torrent file by its relative URL (as returned
// by Find, e.g. "dl.php?t=123"). It returns an error for a non-200 response,
// ErrNotLoggedIn when the session cannot be restored, and ErrNotTorrent when
// the body is not a bencoded dictionary, so HTML pages never reach the
// caller as torrent data.
func (c *RutrackerClient) DownloadTorrent(url string) ([]byte, error) {
	absoluteUrl := c.baseURL + url
	body, err := c.fetch("download", maxTorrentSize, func() (*http.Response, error) {
		return c.httpClient.Get(absoluteUrl)
	})
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || body[0] != 'd' {
		return nil, ErrNotTorrent
	}
	return body, nil
}
