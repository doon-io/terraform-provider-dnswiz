package client

import (
	"context"
	"net/http"
)

// Pool mirrors /v1/pools items. The computed fields (member_count,
// health_score, enabled_up, enabled_total) are read-only on the
// server side.
type Pool struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	HealthMonitorID string `json:"health_monitor_id"`
	SelectionMethod string `json:"selection_method"`
	MemberCount     int    `json:"member_count,omitempty"`
	HealthScore     int    `json:"health_score,omitempty"`
	EnabledUp       int    `json:"enabled_up,omitempty"`
	EnabledTotal    int    `json:"enabled_total,omitempty"`
}

// PoolUpdate is the patch shape; nil pointer means "leave alone".
type PoolUpdate struct {
	Name            *string `json:"name,omitempty"`
	Description     *string `json:"description,omitempty"`
	HealthMonitorID *string `json:"health_monitor_id,omitempty"`
	SelectionMethod *string `json:"selection_method,omitempty"`
}

func (c *Client) CreatePool(ctx context.Context, in Pool) (*Pool, error) {
	var out Pool
	if err := c.Do(ctx, http.MethodPost, "/v1/pools", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPool(ctx context.Context, id string) (*Pool, error) {
	var out Pool
	if err := c.Do(ctx, http.MethodGet, "/v1/pools/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdatePool(ctx context.Context, id string, in PoolUpdate) (*Pool, error) {
	var out Pool
	if err := c.Do(ctx, http.MethodPatch, "/v1/pools/"+id, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeletePool(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/pools/"+id, nil, nil)
}

func (c *Client) ListPools(ctx context.Context) ([]Pool, error) {
	var page struct {
		Items []Pool `json:"items"`
	}
	if err := c.Do(ctx, http.MethodGet, "/v1/pools?limit=500", nil, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

// PoolMember is one endpoint bound into a pool. Membership lifecycle
// (add, modify, remove) is separate from the underlying endpoint's
// lifecycle.
type PoolMember struct {
	ID         string `json:"id,omitempty"`
	PoolID     string `json:"pool_id,omitempty"`
	EndpointID string `json:"endpoint_id"`
	Weight     int    `json:"weight"`
	Priority   int    `json:"priority,omitempty"`
	Enabled    bool   `json:"enabled,omitempty"`
}

type PoolMemberCreate struct {
	EndpointID string `json:"endpoint_id"`
	Weight     int    `json:"weight"`
}

type PoolMemberUpdate struct {
	Weight   *int `json:"weight,omitempty"`
	Priority *int `json:"priority,omitempty"`
}

func (c *Client) AddPoolMember(ctx context.Context, poolID string, in PoolMemberCreate) (*PoolMember, error) {
	var out PoolMember
	if err := c.Do(ctx, http.MethodPost, "/v1/pools/"+poolID+"/members", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPoolMember(ctx context.Context, poolID, memberID string) (*PoolMember, error) {
	var page struct {
		Items []PoolMember `json:"items"`
	}
	if err := c.Do(ctx, http.MethodGet, "/v1/pools/"+poolID+"/members", nil, &page); err != nil {
		return nil, err
	}
	for i := range page.Items {
		if page.Items[i].ID == memberID {
			return &page.Items[i], nil
		}
	}
	return nil, ErrNotFound
}

func (c *Client) UpdatePoolMember(ctx context.Context, poolID, memberID string, in PoolMemberUpdate) (*PoolMember, error) {
	var out PoolMember
	if err := c.Do(ctx, http.MethodPatch, "/v1/pools/"+poolID+"/members/"+memberID, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type poolMemberEnabledReq struct {
	Enabled bool `json:"enabled"`
}

func (c *Client) SetPoolMemberEnabled(ctx context.Context, poolID, memberID string, enabled bool) error {
	return c.Do(ctx, http.MethodPatch, "/v1/pools/"+poolID+"/members/"+memberID+"/enabled", poolMemberEnabledReq{Enabled: enabled}, nil)
}

func (c *Client) RemovePoolMember(ctx context.Context, poolID, memberID string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/pools/"+poolID+"/members/"+memberID, nil, nil)
}
