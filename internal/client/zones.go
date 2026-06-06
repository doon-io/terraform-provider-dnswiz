package client

import (
	"context"
	"net/http"
)

// Zone mirrors the shape of /v1/me/zones items. Only the fields the
// provider reads or writes are listed. Unknown fields in the API
// response are ignored.
type Zone struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	DefaultTTL int    `json:"default_ttl,omitempty"`
	RName      string `json:"rname,omitempty"`
	NegativeTTL int   `json:"negative_ttl,omitempty"`
}

func (c *Client) CreateZone(ctx context.Context, in Zone) (*Zone, error) {
	var out Zone
	if err := c.Do(ctx, http.MethodPost, "/v1/me/zones", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetZone(ctx context.Context, id string) (*Zone, error) {
	var out Zone
	if err := c.Do(ctx, http.MethodGet, "/v1/me/zones/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateZone(ctx context.Context, id string, in Zone) (*Zone, error) {
	var out Zone
	if err := c.Do(ctx, http.MethodPatch, "/v1/me/zones/"+id, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteZone(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/me/zones/"+id, nil, nil)
}
