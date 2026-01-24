package rutracker

import (
	"fmt"
	"io"
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
}

func NewAuthenticatedRutrackerClient(username string, password string) (RutrackerClient, error) {
	httpClient := http.Client{
		Jar:       web.NewJar(),
		Transport: web.DefaultTransport(5000),
		Timeout:   5 * time.Second,
	}

	client := RutrackerClient{
		username:   username,
		password:   password,
		httpClient: &httpClient,
	}

	err := authenticate(&httpClient, username, password)
	if err != nil {
		return client, err
	}
	return client, nil
}

func authenticate(httpClient *http.Client, username string, password string) error {
	rutrackerLoginURL := "https://rutracker.org/forum/login.php"
	form := url.Values{}
	form.Set("login_username", username)
	form.Set("login_password", password)
	form.Set("login", "%C2%F5%EE%E4")
	formData := strings.NewReader(form.Encode())
	res, err := httpClient.Post(rutrackerLoginURL, "application/x-www-form-urlencoded", formData)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return fmt.Errorf("rutracker login failed. Status code: %v", res.StatusCode)
	}
	return nil
}

func (c *RutrackerClient) Find(pattern string) ([]RutrackerSearchItem, error) {
	searchURL := fmt.Sprintf("https://rutracker.org/forum/tracker.php?nm=%s", url.QueryEscape(pattern))
	searchBodyData := url.Values{}
	searchBodyData.Set("nm", pattern)
	searchBody := strings.NewReader(searchBodyData.Encode())
	res, err := c.httpClient.Post(searchURL, "application/x-www-form-urlencoded", searchBody)
	if err != nil {
		logger.Error(err, "Request failed")
		return nil, err
	}
	defer res.Body.Close()

	responseBodyData, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Error(err, "Failed to read response body")
		return nil, err
	}

	items, err := ParseSearchItems(&responseBodyData)
	if err != nil {
		logger.Error(err, "Failed to parse search items")
		return nil, err
	}

	return items, nil
}

func (c *RutrackerClient) DownloadTorrent(url string) ([]byte, error) {
	absoluteUrl := fmt.Sprintf("https://rutracker.org/forum/%v", url)
	res, err := c.httpClient.Get(absoluteUrl)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bodyData, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return bodyData, nil
}
