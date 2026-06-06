// Package client is a thin Go wrapper around the dnswiz REST API.
//
// One Client per provider instance so HTTP keep-alives and the
// connection pool are shared across resources.
package client

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
)

// Client talks to a dnswiz API. Construct via New. The zero value is
// not usable.
type Client struct {
	baseURL    string
	apiKey     string
	userAgent  string
	httpClient *http.Client
}

func New(baseURL, apiKey, userAgent string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		userAgent: userAgent,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ErrNotFound is returned when the API responds 404. Resources check
// for it on Read so they can remove the object from state when it has
// been deleted out of band, rather than failing the plan.
var ErrNotFound = errors.New("dnswiz: not found")

// APIError carries a non-2xx response. The Title, Detail, and Code
// fields come from the server's RFC 7807 problem+json payload when
// the server returns one.
type APIError struct {
	Status int
	Title  string
	Detail string
	Code   string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("dnswiz API %d: %s", e.Status, e.Detail)
	}
	if e.Title != "" {
		return fmt.Sprintf("dnswiz API %d: %s", e.Status, e.Title)
	}
	return fmt.Sprintf("dnswiz API %d", e.Status)
}

// Do performs one HTTP request. If body is non-nil it is JSON-encoded.
// If out is non-nil the response body is JSON-decoded into it. A 404
// response is returned as ErrNotFound; other non-2xx responses are
// returned as *APIError.
func (c *Client) Do(ctx context.Context, method, p string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	u, err := url.JoinPath(c.baseURL, p)
	if err != nil {
		return fmt.Errorf("build url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseProblem(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func parseProblem(resp *http.Response) error {
	apiErr := &APIError{Status: resp.StatusCode}
	var problem struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Code   string `json:"code"`
	}
	body, _ := io.ReadAll(resp.Body)
	if json.Unmarshal(body, &problem) == nil {
		apiErr.Title = problem.Title
		apiErr.Detail = problem.Detail
		apiErr.Code = problem.Code
	}
	return apiErr
}
