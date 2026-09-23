// Package rj is a client for the Radio Javan web API, the same API that
// play.radiojavan.com uses. Everything that is public on the site works
// without an account; a session cookie is only needed for personal data
// (liked songs, library, own playlists) and for RJ Premium content.
package rj

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MahdiGraph/radio-javan-downloader/internal/netutil"
)

const (
	// SiteURL is the web player that owns the API.
	SiteURL = "https://play.radiojavan.com"

	apiPath = "/api/p/"

	// apiKey and rjUserAgent mirror the headers the web player sends with
	// every API call.
	apiKey      = "40e87948bd4ef75efe61205ac5f468a9fd2b970511acf58c49706ecb984f1d67"
	rjUserAgent = "Radio Javan/5.1.0 (Web) com.radioJavan.rj.web"

	// BrowserUserAgent is sent as the User-Agent of every request.
	BrowserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

	// SessionCookie is the cookie that carries a logged-in session.
	SessionCookie = "_rj_web"

	maxResponseSize = 64 << 20
)

var (
	// ErrNotFound is returned when Radio Javan has nothing for an id.
	ErrNotFound = errors.New("not found on Radio Javan")
	// ErrLoginRequired is returned by endpoints that need a session.
	ErrLoginRequired = errors.New("this needs a Radio Javan account: run `rjdl login` first")
	// ErrPremiumRequired is returned for RJ Premium content when the
	// session has no Premium subscription.
	ErrPremiumRequired = errors.New("this is RJ Premium content: log in with a Premium account (`rjdl login`)")
)

// APIError is an error reported by the API.
type APIError struct {
	Endpoint string
	Status   int
	Message  string
}

func (e *APIError) Error() string {
	if e.Status != 0 && e.Status != http.StatusOK {
		return fmt.Sprintf("radio javan api %s: HTTP %d: %s", e.Endpoint, e.Status, e.Message)
	}
	return fmt.Sprintf("radio javan api %s: %s", e.Endpoint, e.Message)
}

// Client talks to the Radio Javan API. The zero value is not usable; call
// NewClient.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	// Cookie is sent as the Cookie header of API requests. It is empty for
	// anonymous use; see SessionHeader.
	Cookie string
}

// NewClient returns an anonymous client.
func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		BaseURL: SiteURL,
	}
}

// LoggedIn reports whether the client carries a session.
func (c *Client) LoggedIn() bool { return c.Cookie != "" }

// SessionHeader turns a user supplied session into a Cookie header value.
// It accepts the bare value of the _rj_web cookie or a whole
// "name=value; name2=value2" header as copied from a browser.
func SessionHeader(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 7 && strings.EqualFold(s[:7], "cookie:") {
		s = strings.TrimSpace(s[7:])
	}
	if s == "" || strings.Contains(s, "=") {
		return s
	}
	return SessionCookie + "=" + s
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, params url.Values, body any) (*http.Request, error) {
	u := strings.TrimRight(c.BaseURL, "/") + apiPath + strings.TrimLeft(endpoint, "/")
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", BrowserUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("x-rj-user-agent", rjUserAgent)
	req.Header.Set("x-rj-proxy-client", "client")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Cookie != "" {
		req.Header.Set("Cookie", c.Cookie)
	}
	return req, nil
}

// call performs an API request, retrying rate limits, server errors and
// network failures, and returns the response with its body.
func (c *Client) call(ctx context.Context, method, endpoint string, params url.Values, body any) (*http.Response, []byte, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			}
		}
		req, err := c.newRequest(ctx, method, endpoint, params, body)
		if err != nil {
			return nil, nil, err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil || netutil.Filtered(err) {
				return nil, nil, err
			}
			lastErr = err
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &APIError{Endpoint: endpoint, Status: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			msg := http.StatusText(resp.StatusCode)
			if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
				msg += " (blocked by Cloudflare? try again later or from another network)"
			} else if m, ok := failureMessage(data); ok {
				msg = m
			}
			return resp, data, &APIError{Endpoint: endpoint, Status: resp.StatusCode, Message: msg}
		}
		if msg, failed := failureMessage(data); failed {
			return resp, data, &APIError{Endpoint: endpoint, Status: resp.StatusCode, Message: msg}
		}
		return resp, data, nil
	}
	return nil, nil, lastErr
}

// failureMessage mirrors the web player: a response is an error when it has
// "success": false and either nothing else or a "msg".
func failureMessage(data []byte) (string, bool) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return "", false
	}
	var probe map[string]json.RawMessage
	if json.Unmarshal(data, &probe) != nil {
		return "", false
	}
	if raw, ok := probe["success"]; !ok || string(raw) != "false" {
		return "", false
	}
	var msg string
	if raw, ok := probe["msg"]; ok {
		_ = json.Unmarshal(raw, &msg)
	}
	if len(probe) == 1 || msg != "" {
		if msg == "" {
			msg = "request was rejected"
		}
		return msg, true
	}
	return "", false
}

func (c *Client) getJSON(ctx context.Context, endpoint string, params url.Values, out any) error {
	_, data, err := c.call(ctx, http.MethodGet, endpoint, params, nil)
	if err != nil {
		return err
	}
	return decode(endpoint, data, out)
}

// getRaw returns the undecoded response of a GET request.
func (c *Client) getRaw(ctx context.Context, endpoint string, params url.Values) (json.RawMessage, error) {
	_, data, err := c.call(ctx, http.MethodGet, endpoint, params, nil)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("radio javan api %s: response is not JSON", endpoint)
	}
	return json.RawMessage(data), nil
}

// decode unmarshals an API response. The API is not consistent about field
// types (ids are sometimes numbers and sometimes strings, some fields are
// objects in one endpoint and strings in another), so fields that do not fit
// are skipped instead of failing the whole response.
func decode(endpoint string, data []byte, out any) error {
	err := json.Unmarshal(data, out)
	var typeErr *json.UnmarshalTypeError
	if err != nil && !errors.As(err, &typeErr) {
		return fmt.Errorf("radio javan api %s: unexpected response: %w", endpoint, err)
	}
	return nil
}
