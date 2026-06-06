package client

import (
	"context"
	"encoding/json"
	"net/http"
)

// Record mirrors /v1/records items. The data envelope's shape depends
// on the record type; callers build it via the record-type helpers
// below.
type Record struct {
	ID         string          `json:"id,omitempty"`
	ZoneID     string          `json:"zone_id,omitempty"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	TTL        int             `json:"ttl"`
	TTLInherit bool            `json:"ttl_inherit"`
	Data       json.RawMessage `json:"data"`
	Active     bool            `json:"active"`
	Comment    string          `json:"comment,omitempty"`
}

// RecordUpdate is the patch shape. Pointer fields are nil to leave
// alone.
type RecordUpdate struct {
	Name       *string         `json:"name,omitempty"`
	TTL        *int            `json:"ttl,omitempty"`
	TTLInherit *bool           `json:"ttl_inherit,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	Active     *bool           `json:"active,omitempty"`
	Comment    *string         `json:"comment,omitempty"`
}

func (c *Client) CreateRecord(ctx context.Context, zoneID string, in Record) (*Record, error) {
	var out Record
	if err := c.Do(ctx, http.MethodPost, "/v1/zones/"+zoneID+"/records", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRecord(ctx context.Context, id string) (*Record, error) {
	var out Record
	if err := c.Do(ctx, http.MethodGet, "/v1/records/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateRecord(ctx context.Context, id string, in RecordUpdate) (*Record, error) {
	var out Record
	if err := c.Do(ctx, http.MethodPatch, "/v1/records/"+id, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteRecord(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/records/"+id, nil, nil)
}
