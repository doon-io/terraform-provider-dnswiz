package client

import (
	"context"
	"net/http"
)

// NotificationChannel mirrors /v1/notifications items. The signing
// secret is returned once at create and never again; persist it
// somewhere safe.
type NotificationChannel struct {
	ID     string   `json:"id,omitempty"`
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Target string   `json:"target"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events"`
	Active bool     `json:"active,omitempty"`
}

type NotificationChannelUpdate struct {
	Name   *string  `json:"name,omitempty"`
	Target *string  `json:"target,omitempty"`
	Events []string `json:"events,omitempty"`
	Active *bool    `json:"active,omitempty"`
}

func (c *Client) CreateNotificationChannel(ctx context.Context, in NotificationChannel) (*NotificationChannel, error) {
	var out NotificationChannel
	if err := c.Do(ctx, http.MethodPost, "/v1/notifications", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetNotificationChannel(ctx context.Context, id string) (*NotificationChannel, error) {
	var out NotificationChannel
	if err := c.Do(ctx, http.MethodGet, "/v1/notifications/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateNotificationChannel(ctx context.Context, id string, in NotificationChannelUpdate) (*NotificationChannel, error) {
	var out NotificationChannel
	if err := c.Do(ctx, http.MethodPatch, "/v1/notifications/"+id, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteNotificationChannel(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/notifications/"+id, nil, nil)
}
