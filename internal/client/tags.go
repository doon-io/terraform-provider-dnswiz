package client

import (
	"context"
	"net/http"
	"sort"
)

type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (c *Client) ListTags(ctx context.Context) ([]Tag, error) {
	var page struct {
		Items []Tag `json:"items"`
	}
	if err := c.Do(ctx, http.MethodGet, "/v1/tags", nil, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

// EnsureTags resolves tag names to IDs, creating any that don't exist yet, and
// returns the IDs in the same order as the input names.
func (c *Client) EnsureTags(ctx context.Context, names []string) ([]string, error) {
	existing, err := c.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]string, len(existing))
	for _, t := range existing {
		byName[t.Name] = t.ID
	}
	ids := make([]string, 0, len(names))
	for _, n := range names {
		id, ok := byName[n]
		if !ok {
			var created Tag
			if err := c.Do(ctx, http.MethodPost, "/v1/tags", map[string]string{"name": n, "color": "slate"}, &created); err != nil {
				return nil, err
			}
			id = created.ID
			byName[n] = id
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// AssignTags sets the full tag set on a resource (replaces any existing).
func (c *Client) AssignTags(ctx context.Context, resourceType, resourceID string, tagIDs []string) error {
	body := map[string]any{"resource_type": resourceType, "resource_id": resourceID, "tag_ids": tagIDs}
	return c.Do(ctx, http.MethodPut, "/v1/tags/assign", body, nil)
}

// TagNamesForResource returns the sorted tag names attached to one resource.
func (c *Client) TagNamesForResource(ctx context.Context, resourceType, id string) ([]string, error) {
	body := map[string]any{"resource_type": resourceType, "ids": []string{id}}
	var resp struct {
		Tags map[string][]Tag `json:"tags"`
	}
	if err := c.Do(ctx, http.MethodPost, "/v1/tags/for", body, &resp); err != nil {
		return nil, err
	}
	var names []string
	for _, t := range resp.Tags[id] {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names, nil
}
