package client

import (
	"context"
	"net/http"
)

// Zone mirrors the shape of /v1/zones items. Pointer fields are
// nullable on the server side: a nil pointer means "inherit the
// tenant default", and a non-nil pointer is the explicit override.
type Zone struct {
	ID          string  `json:"id,omitempty"`
	Name        string  `json:"name"`
	Active      bool    `json:"active"`
	DefaultTTL  *int    `json:"default_ttl,omitempty"`
	SOARName    *string `json:"soa_rname,omitempty"`
	NegativeTTL *int    `json:"negative_ttl,omitempty"`
}

// ZoneCreate is the create-request shape. Only the apex name is
// required; everything else inherits from the tenant default.
type ZoneCreate struct {
	Name string `json:"name"`
}

// ZoneUpdate is the patch-request shape. Pointer fields are nil to
// leave the value alone. To clear a value to its tenant default, send
// a negative number (TTL fields) or empty string (soa_rname).
type ZoneUpdate struct {
	Active      *bool   `json:"active,omitempty"`
	DefaultTTL  *int    `json:"default_ttl,omitempty"`
	SOARName    *string `json:"soa_rname,omitempty"`
	NegativeTTL *int    `json:"negative_ttl,omitempty"`
}

func (c *Client) CreateZone(ctx context.Context, in ZoneCreate) (*Zone, error) {
	var out Zone
	if err := c.Do(ctx, http.MethodPost, "/v1/zones", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetZone(ctx context.Context, id string) (*Zone, error) {
	var out Zone
	if err := c.Do(ctx, http.MethodGet, "/v1/zones/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateZone(ctx context.Context, id string, in ZoneUpdate) (*Zone, error) {
	var out Zone
	if err := c.Do(ctx, http.MethodPatch, "/v1/zones/"+id, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteZone(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/zones/"+id, nil, nil)
}

// ListZones returns the first page of zones. For tenants with many
// zones the next_cursor is ignored; data sources are expected to look
// up by unique name.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var page struct {
		Items []Zone `json:"items"`
	}
	if err := c.Do(ctx, http.MethodGet, "/v1/zones?limit=500", nil, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}
