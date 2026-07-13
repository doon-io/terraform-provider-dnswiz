package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                = (*ipamNetworkResource)(nil)
	_ resource.ResourceWithConfigure   = (*ipamNetworkResource)(nil)
	_ resource.ResourceWithImportState = (*ipamNetworkResource)(nil)
)

func NewIPAMNetworkResource() resource.Resource { return &ipamNetworkResource{} }

type ipamNetworkResource struct {
	client *client.Client
}

type ipamNetworkResourceModel struct {
	ID            types.String `tfsdk:"id"`
	VRFID         types.String `tfsdk:"vrf_id"`
	CIDR          types.String `tfsdk:"cidr"`
	ParentBlockID types.String `tfsdk:"parent_block_id"`
	ParentTags    types.Set    `tfsdk:"parent_tags"`
	PrefixLength  types.Int64  `tfsdk:"prefix_length"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	GatewayIP     types.String `tfsdk:"gateway_ip"`
	Tags          types.Set    `tfsdk:"tags"`
	Family        types.Int64  `tfsdk:"family"`
	Version       types.Int64  `tfsdk:"version"`
}

func (r *ipamNetworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipam_network"
}

func (r *ipamNetworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An IPAM network (operational subnet). Choose exactly one of three ways to place it: an explicit `cidr` (with `parent_block_id`), `prefix_length` + `parent_block_id` to take the next free subnet of that size from one block, or `prefix_length` + `parent_tags` to take it from the first block (by CIDR) carrying all those tags — a prefix pool, e.g. \"a /24 from region:eu-central-1\".",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Network ID assigned by dnswiz.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cidr": schema.StringAttribute{
				MarkdownDescription: "The network's CIDR. Set it explicitly, or leave unset and let an allocation mode compute it. Changing an explicit value forces a new resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"parent_block_id": schema.StringAttribute{
				MarkdownDescription: "Block that contains the network. Required with an explicit `cidr`; with `prefix_length` it's the block to carve from; with `parent_tags` it's computed. Forces a new resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"parent_tags": schema.SetAttribute{
				MarkdownDescription: "Allocate from the pool of blocks carrying ALL of these tags. Use with `prefix_length`. Forces a new resource.",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.Set{setplanmodifier.RequiresReplace()},
			},
			"prefix_length": schema.Int64Attribute{
				MarkdownDescription: "Desired subnet size (e.g. `24`) when auto-allocating. Omit when `cidr` is explicit. Forces a new resource.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-friendly name.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"gateway_ip": schema.StringAttribute{
				MarkdownDescription: "Optional gateway address inside the network.",
				Optional:            true,
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Tag names to attach to the network. Missing tags are created.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"vrf_id": schema.StringAttribute{
				MarkdownDescription: "VRF for an explicit `cidr` network. Omit to use the default VRF; ignored when auto-allocating (the block's VRF wins). Forces a new resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"family": schema.Int64Attribute{
				MarkdownDescription: "Address family (4 or 6).",
				Computed:            true,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "Optimistic-concurrency version.",
				Computed:            true,
			},
		},
	}
}

func (r *ipamNetworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *ipamNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ipamNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var net *client.IPAMNetwork
	var err error

	switch {
	case !plan.CIDR.IsNull() && !plan.CIDR.IsUnknown():
		if plan.ParentBlockID.IsNull() || plan.ParentBlockID.IsUnknown() {
			resp.Diagnostics.AddError("missing parent_block_id", "an explicit cidr network requires parent_block_id")
			return
		}
		vrf := plan.VRFID.ValueString()
		if vrf == "" {
			vrf, err = r.client.DefaultVRFID(ctx)
			if err != nil {
				resp.Diagnostics.AddError("resolve default VRF", err.Error())
				return
			}
		}
		net, err = r.client.CreateNetwork(ctx, client.IPAMNetworkCreate{
			VRFID:         vrf,
			ParentBlockID: plan.ParentBlockID.ValueString(),
			CIDR:          plan.CIDR.ValueString(),
			Name:          plan.Name.ValueString(),
			Description:   plan.Description.ValueString(),
			GatewayIP:     optString(plan.GatewayIP),
		})
	case !plan.ParentTags.IsNull() && !plan.PrefixLength.IsNull():
		var tagNames []string
		resp.Diagnostics.Append(plan.ParentTags.ElementsAs(ctx, &tagNames, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		net, err = r.client.AllocateNetworkFromPool(ctx, tagNames, int(plan.PrefixLength.ValueInt64()), plan.Name.ValueString())
	case !plan.ParentBlockID.IsNull() && !plan.PrefixLength.IsNull():
		net, err = r.client.AllocateNetworkFromBlock(ctx, plan.ParentBlockID.ValueString(), int(plan.PrefixLength.ValueInt64()), plan.Name.ValueString())
	default:
		resp.Diagnostics.AddError("no placement given", "set cidr (with parent_block_id), or prefix_length with parent_block_id or parent_tags")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("create ipam network", err.Error())
		return
	}

	// Allocation endpoints only take name — apply description/gateway after.
	if net.Description != plan.Description.ValueString() || (optString(plan.GatewayIP) != nil) {
		net, err = r.client.UpdateNetwork(ctx, net.ID, net.Version, client.IPAMNetworkUpdate{
			Name:        plan.Name.ValueString(),
			Description: plan.Description.ValueString(),
			GatewayIP:   optString(plan.GatewayIP),
		})
		if err != nil {
			resp.Diagnostics.AddError("apply network settings", err.Error())
			return
		}
	}

	if diag := r.syncTags(ctx, net.ID, plan.Tags); diag != nil {
		resp.Diagnostics.Append(diag...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, r.modelFrom(ctx, net, plan, plan.Tags))...)
}

func (r *ipamNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ipamNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetNetwork(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read ipam network", err.Error())
		return
	}
	names, err := r.client.TagNamesForResource(ctx, "network", got.ID)
	if err != nil {
		resp.Diagnostics.AddError("read network tags", err.Error())
		return
	}
	tagSet, d := tagsToSet(ctx, state.Tags, names)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.modelFrom(ctx, got, state, tagSet))...)
}

func (r *ipamNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ipamNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cur, err := r.client.GetNetwork(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("read network for update", err.Error())
		return
	}
	got, err := r.client.UpdateNetwork(ctx, plan.ID.ValueString(), cur.Version, client.IPAMNetworkUpdate{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		GatewayIP:   optString(plan.GatewayIP),
	})
	if err != nil {
		resp.Diagnostics.AddError("update ipam network", err.Error())
		return
	}
	if diag := r.syncTags(ctx, got.ID, plan.Tags); diag != nil {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.modelFrom(ctx, got, plan, plan.Tags))...)
}

func (r *ipamNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ipamNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNetwork(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete ipam network", err.Error())
		return
	}
}

func (r *ipamNetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// syncTags resolves the desired tag names to IDs (creating missing) and sets
// them on the network. A null/unknown set leaves tags untouched only when it's
// also absent server-side; here we treat null as "no tags".
func (r *ipamNetworkResource) syncTags(ctx context.Context, id string, want types.Set) diag.Diagnostics {
	return assignResourceTags(ctx, r.client, "network", id, want)
}

// assignResourceTags resolves the desired tag names to IDs (creating any that
// don't exist) and sets them as the full tag set on a resource. A null/unknown
// set clears tags.
func assignResourceTags(ctx context.Context, c *client.Client, resourceType, id string, want types.Set) diag.Diagnostics {
	var names []string
	if !want.IsNull() && !want.IsUnknown() {
		if d := want.ElementsAs(ctx, &names, false); d.HasError() {
			return d
		}
	}
	ids, err := c.EnsureTags(ctx, names)
	if err != nil {
		var d diag.Diagnostics
		d.AddError("resolve tags", err.Error())
		return d
	}
	if err := c.AssignTags(ctx, resourceType, id, ids); err != nil {
		var d diag.Diagnostics
		d.AddError("assign tags", err.Error())
		return d
	}
	return nil
}

// modelFrom builds the resource state from the API network. cfg carries the
// config-only inputs (parent_tags, prefix_length) that the API doesn't echo,
// so they must round-trip verbatim from plan/state or Terraform errors on an
// inconsistent result.
func (r *ipamNetworkResource) modelFrom(ctx context.Context, n *client.IPAMNetwork, cfg ipamNetworkResourceModel, tagSet types.Set) ipamNetworkResourceModel {
	m := ipamNetworkResourceModel{
		ID:            types.StringValue(n.ID),
		VRFID:         types.StringValue(n.VRFID),
		CIDR:          types.StringValue(n.CIDR),
		ParentBlockID: types.StringValue(n.ParentBlockID),
		Name:          types.StringValue(n.Name),
		Description:   types.StringValue(n.Description),
		Family:        types.Int64Value(int64(n.Family)),
		Version:       types.Int64Value(int64(n.Version)),
		Tags:          tagSet,
		ParentTags:    cfg.ParentTags,
		PrefixLength:  cfg.PrefixLength,
	}
	if n.GatewayIP != nil {
		m.GatewayIP = types.StringValue(*n.GatewayIP)
	} else {
		m.GatewayIP = types.StringNull()
	}
	return m
}

// optString maps a Terraform string to *string: nil when null/unknown/empty.
func optString(s types.String) *string {
	if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
		return nil
	}
	v := s.ValueString()
	return &v
}

// tagsToSet builds the state tag set from the server's tag names. Empty →
// null so a resource with no tags configured doesn't churn against an empty
// set. `prior` is accepted for symmetry with callers but not needed here.
func tagsToSet(ctx context.Context, prior types.Set, names []string) (types.Set, diag.Diagnostics) {
	_ = prior
	if len(names) == 0 {
		return types.SetNull(types.StringType), nil
	}
	return types.SetValueFrom(ctx, types.StringType, names)
}
