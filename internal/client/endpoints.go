package client

import (
	"context"
	"net/http"
)

// Endpoint mirrors /v1/endpoints items. When HealthMonitorID is set,
// the probe target is composed from monitor + host + port. Without a
// monitor, kind + target is the legacy fallback. Target is required
// by the create endpoint even when a monitor is set; the provider
// fills it from value if the user didn't set it explicitly.
type Endpoint struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Target          string `json:"target"`
	Value           string `json:"value,omitempty"`
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty"`
	HealthMonitorID string `json:"health_monitor_id,omitempty"`
	ExpectedStatus  int    `json:"expected_status,omitempty"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
	HealthyAfter    int    `json:"healthy_after,omitempty"`
	UnhealthyAfter  int    `json:"unhealthy_after,omitempty"`
}

func (c *Client) CreateEndpoint(ctx context.Context, in Endpoint) (*Endpoint, error) {
	var out Endpoint
	if err := c.Do(ctx, http.MethodPost, "/v1/endpoints", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetEndpoint(ctx context.Context, id string) (*Endpoint, error) {
	var out Endpoint
	if err := c.Do(ctx, http.MethodGet, "/v1/endpoints/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateEndpoint(ctx context.Context, id string, in Endpoint) (*Endpoint, error) {
	var out Endpoint
	if err := c.Do(ctx, http.MethodPatch, "/v1/endpoints/"+id, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteEndpoint(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/endpoints/"+id, nil, nil)
}

func (c *Client) ListEndpoints(ctx context.Context) ([]Endpoint, error) {
	var page struct {
		Items []Endpoint `json:"items"`
	}
	if err := c.Do(ctx, http.MethodGet, "/v1/endpoints?limit=500", nil, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}
