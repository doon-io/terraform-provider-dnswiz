package client

import (
	"context"
	"net/http"
)

// HealthMonitor mirrors /v1/health-monitors items. Presets owned by
// dnswiz cannot be modified or deleted; user-created monitors can.
type HealthMonitor struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	IsPreset        bool   `json:"is_preset,omitempty"`
	Kind            string `json:"kind"`
	Path            string `json:"path,omitempty"`
	ExpectedStatus  int    `json:"expected_status,omitempty"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
	HealthyAfter    int    `json:"healthy_after,omitempty"`
	UnhealthyAfter  int    `json:"unhealthy_after,omitempty"`
}

func (c *Client) CreateHealthMonitor(ctx context.Context, in HealthMonitor) (*HealthMonitor, error) {
	var out HealthMonitor
	if err := c.Do(ctx, http.MethodPost, "/v1/health-monitors", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetHealthMonitor(ctx context.Context, id string) (*HealthMonitor, error) {
	var out HealthMonitor
	if err := c.Do(ctx, http.MethodGet, "/v1/health-monitors/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateHealthMonitor(ctx context.Context, id string, in HealthMonitor) (*HealthMonitor, error) {
	var out HealthMonitor
	if err := c.Do(ctx, http.MethodPatch, "/v1/health-monitors/"+id, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteHealthMonitor(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/health-monitors/"+id, nil, nil)
}
