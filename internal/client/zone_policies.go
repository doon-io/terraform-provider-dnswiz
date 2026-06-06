package client

import (
	"context"
	"encoding/json"
	"net/http"
)

// ZonePolicy mirrors a single row from /v1/zones/{zone_id}/security.
// Config shape depends on Kind; the provider hands the raw JSON
// through without trying to parse it server-side.
type ZonePolicy struct {
	Kind    string          `json:"kind"`
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config"`
}

// ZonePolicyUpdate is what we PATCH. Nil fields are left alone server
// side, so the caller can flip enabled without touching config and
// vice versa.
type ZonePolicyUpdate struct {
	Enabled *bool            `json:"enabled,omitempty"`
	Config  *json.RawMessage `json:"config,omitempty"`
}

func (c *Client) ListZonePolicies(ctx context.Context, zoneID string) ([]ZonePolicy, error) {
	var out []ZonePolicy
	if err := c.Do(ctx, http.MethodGet, "/v1/zones/"+zoneID+"/security", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) PatchZonePolicy(ctx context.Context, zoneID, kind string, in ZonePolicyUpdate) (*ZonePolicy, error) {
	var out ZonePolicy
	if err := c.Do(ctx, http.MethodPatch, "/v1/zones/"+zoneID+"/security/"+kind, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
