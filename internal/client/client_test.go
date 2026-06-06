package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoSetsAuthAndUserAgent(t *testing.T) {
	var gotAuth, gotUA, gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "secret", "test-ua/0.1")
	if err := c.Do(context.Background(), http.MethodPost, "/v1/me/zones", map[string]string{"name": "x"}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret")
	}
	if gotUA != "test-ua/0.1" {
		t.Errorf("User-Agent = %q, want %q", gotUA, "test-ua/0.1")
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotCT, "application/json")
	}
}

func TestDoReturnsErrNotFoundOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "k", "")
	err := c.Do(context.Background(), http.MethodGet, "/v1/me/zones/x", nil, nil)
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDoParsesProblemJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"title":  "bad request",
			"detail": "name is required",
			"code":   "validation_error",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "k", "")
	err := c.Do(context.Background(), http.MethodPost, "/v1/me/zones", map[string]string{}, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", apiErr.Status)
	}
	if apiErr.Detail != "name is required" {
		t.Errorf("Detail = %q", apiErr.Detail)
	}
	if !strings.Contains(apiErr.Error(), "name is required") {
		t.Errorf("Error() = %q, want it to include detail", apiErr.Error())
	}
}

func TestDoDecodesResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"z1","name":"example.com"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k", "")
	var z Zone
	if err := c.Do(context.Background(), http.MethodGet, "/v1/me/zones/z1", nil, &z); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if z.ID != "z1" || z.Name != "example.com" {
		t.Errorf("zone = %+v", z)
	}
}
