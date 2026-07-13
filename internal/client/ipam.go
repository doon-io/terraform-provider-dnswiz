package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// IPAMBlock is a planning-layer prefix in the address-space hierarchy. Its
// place in the tree is derived from CIDR containment server-side (no manual
// parent), so parent_block_id is read-only.
type IPAMBlock struct {
	ID            string  `json:"id,omitempty"`
	VRFID         string  `json:"vrf_id,omitempty"`
	ParentBlockID *string `json:"parent_block_id,omitempty"`
	Family        int     `json:"family,omitempty"`
	CIDR          string  `json:"cidr"`
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	Origin        string  `json:"origin,omitempty"`
	Version       int     `json:"version,omitempty"`
}

type IPAMBlockCreate struct {
	VRFID       string `json:"vrf_id"`
	CIDR        string `json:"cidr"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Origin      string `json:"origin"`
}

type IPAMBlockUpdate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Origin      string `json:"origin"`
}

func (c *Client) CreateBlock(ctx context.Context, in IPAMBlockCreate) (*IPAMBlock, error) {
	var out IPAMBlock
	if err := c.Do(ctx, http.MethodPost, "/v1/ipam/blocks", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetBlock(ctx context.Context, id string) (*IPAMBlock, error) {
	var out IPAMBlock
	if err := c.Do(ctx, http.MethodGet, "/v1/ipam/blocks/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateBlock(ctx context.Context, id string, version int, in IPAMBlockUpdate) (*IPAMBlock, error) {
	var out IPAMBlock
	p := "/v1/ipam/blocks/" + id + "?version=" + strconv.Itoa(version)
	if err := c.Do(ctx, http.MethodPatch, p, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteBlock removes a block. mode=dissolve (default) re-homes any children to
// the block's parent rather than deleting unmanaged prefixes.
func (c *Client) DeleteBlock(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/ipam/blocks/"+id+"?mode=dissolve", nil, nil)
}

// IPAMVRF is a routing-domain scope. Every tenant has a default (global) VRF,
// created lazily server-side; blocks/networks belong to exactly one.
type IPAMVRF struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

// DefaultVRFID returns the tenant's default VRF id, used when a block/network
// resource doesn't pin one explicitly.
func (c *Client) DefaultVRFID(ctx context.Context) (string, error) {
	var page struct {
		Items []IPAMVRF `json:"items"`
	}
	if err := c.Do(ctx, http.MethodGet, "/v1/ipam/vrfs", nil, &page); err != nil {
		return "", err
	}
	for _, v := range page.Items {
		if v.IsDefault {
			return v.ID, nil
		}
	}
	if len(page.Items) > 0 {
		return page.Items[0].ID, nil
	}
	return "", fmt.Errorf("no VRF found for tenant")
}

// IPAMNetwork is an operational subnet inside a block. Unlike blocks its parent
// is explicit (set on create or chosen by the allocator), not derived.
type IPAMNetwork struct {
	ID            string  `json:"id,omitempty"`
	VRFID         string  `json:"vrf_id,omitempty"`
	ParentBlockID string  `json:"parent_block_id,omitempty"`
	Family        int     `json:"family,omitempty"`
	CIDR          string  `json:"cidr"`
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	GatewayIP     *string `json:"gateway_ip,omitempty"`
	Version       int     `json:"version,omitempty"`
}

type IPAMNetworkCreate struct {
	VRFID         string  `json:"vrf_id"`
	ParentBlockID string  `json:"parent_block_id"`
	CIDR          string  `json:"cidr"`
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	GatewayIP     *string `json:"gateway_ip,omitempty"`
}

type IPAMNetworkUpdate struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	GatewayIP   *string `json:"gateway_ip"`
}

func (c *Client) CreateNetwork(ctx context.Context, in IPAMNetworkCreate) (*IPAMNetwork, error) {
	var out IPAMNetwork
	if err := c.Do(ctx, http.MethodPost, "/v1/ipam/networks", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllocateNetworkFromBlock carves the next free /mask subnet out of one block.
func (c *Client) AllocateNetworkFromBlock(ctx context.Context, blockID string, mask int, name string) (*IPAMNetwork, error) {
	var out IPAMNetwork
	body := map[string]any{"mask": mask, "name": name}
	if err := c.Do(ctx, http.MethodPost, "/v1/ipam/blocks/"+blockID+"/allocate-network", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllocateNetworkFromPool carves the next free /mask subnet from any block
// carrying all of the given tags (lowest-CIDR block with room wins).
func (c *Client) AllocateNetworkFromPool(ctx context.Context, tags []string, mask int, name string) (*IPAMNetwork, error) {
	var out IPAMNetwork
	body := map[string]any{"tags": tags, "mask": mask, "name": name}
	if err := c.Do(ctx, http.MethodPost, "/v1/ipam/networks/allocate", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetNetwork(ctx context.Context, id string) (*IPAMNetwork, error) {
	var out IPAMNetwork
	if err := c.Do(ctx, http.MethodGet, "/v1/ipam/networks/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateNetwork(ctx context.Context, id string, version int, in IPAMNetworkUpdate) (*IPAMNetwork, error) {
	var out IPAMNetwork
	p := "/v1/ipam/networks/" + id + "?version=" + strconv.Itoa(version)
	if err := c.Do(ctx, http.MethodPatch, p, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteNetwork(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/ipam/networks/"+id, nil, nil)
}

// NextAvailableSubnet peeks the next free aligned /mask prefix inside a block
// WITHOUT allocating it — powers the dnswiz_ipam_available_subnet data source.
func (c *Client) NextAvailableSubnet(ctx context.Context, blockID string, mask int) (string, error) {
	var out struct {
		CIDR string `json:"cidr"`
	}
	p := "/v1/ipam/blocks/" + blockID + "/next-available?mask=" + strconv.Itoa(mask)
	if err := c.Do(ctx, http.MethodGet, p, nil, &out); err != nil {
		return "", err
	}
	return out.CIDR, nil
}
