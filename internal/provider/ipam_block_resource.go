package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                = (*ipamBlockResource)(nil)
	_ resource.ResourceWithConfigure   = (*ipamBlockResource)(nil)
	_ resource.ResourceWithImportState = (*ipamBlockResource)(nil)
)

func NewIPAMBlockResource() resource.Resource { return &ipamBlockResource{} }

type ipamBlockResource struct {
	client *client.Client
}

type ipamBlockResourceModel struct {
	ID            types.String `tfsdk:"id"`
	VRFID         types.String `tfsdk:"vrf_id"`
	CIDR          types.String `tfsdk:"cidr"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	Origin        types.String `tfsdk:"origin"`
	Family        types.Int64  `tfsdk:"family"`
	ParentBlockID types.String `tfsdk:"parent_block_id"`
	Version       types.Int64  `tfsdk:"version"`
}

func (r *ipamBlockResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipam_block"
}

func (r *ipamBlockResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An IPAM block: a planning-layer prefix. dnswiz places it in the address-space tree automatically by CIDR containment, so there is no `parent_block_id` to set — declare a supernet and the prefixes it covers nest underneath it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Block ID assigned by dnswiz.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cidr": schema.StringAttribute{
				MarkdownDescription: "The block's CIDR, for example `10.0.0.0/8`. Write a normalized network address (host bits zero). Changing it forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-friendly name for the block.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"origin": schema.StringAttribute{
				MarkdownDescription: "Provenance of the space: `rfc1918`, `rir-arin`, `rir-ripe`, `rir-apnic`, `rir-lacnic`, `rir-afrinic`, or `other`. Defaults to `rfc1918`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("rfc1918"),
			},
			"vrf_id": schema.StringAttribute{
				MarkdownDescription: "VRF the block belongs to. Omit to use the tenant's default VRF. Changing it forces a new resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"family": schema.Int64Attribute{
				MarkdownDescription: "Address family (4 or 6), derived from the CIDR.",
				Computed:            true,
			},
			"parent_block_id": schema.StringAttribute{
				MarkdownDescription: "The containing block, computed by dnswiz from CIDR containment. Empty when the block is top-level.",
				Computed:            true,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "Optimistic-concurrency version.",
				Computed:            true,
			},
		},
	}
}

func (r *ipamBlockResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ipamBlockResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ipamBlockResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vrf := plan.VRFID.ValueString()
	if vrf == "" {
		id, err := r.client.DefaultVRFID(ctx)
		if err != nil {
			resp.Diagnostics.AddError("resolve default VRF", err.Error())
			return
		}
		vrf = id
	}

	created, err := r.client.CreateBlock(ctx, client.IPAMBlockCreate{
		VRFID:       vrf,
		CIDR:        plan.CIDR.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Origin:      plan.Origin.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("create ipam block", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIBlock(created))...)
}

func (r *ipamBlockResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ipamBlockResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetBlock(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read ipam block", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIBlock(got))...)
}

func (r *ipamBlockResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ipamBlockResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Re-read for a fresh optimistic-concurrency version before the write.
	cur, err := r.client.GetBlock(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("read ipam block for update", err.Error())
		return
	}
	got, err := r.client.UpdateBlock(ctx, plan.ID.ValueString(), cur.Version, client.IPAMBlockUpdate{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Origin:      plan.Origin.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("update ipam block", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIBlock(got))...)
}

func (r *ipamBlockResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ipamBlockResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteBlock(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete ipam block", err.Error())
		return
	}
}

func (r *ipamBlockResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func fromAPIBlock(b *client.IPAMBlock) ipamBlockResourceModel {
	m := ipamBlockResourceModel{
		ID:          types.StringValue(b.ID),
		VRFID:       types.StringValue(b.VRFID),
		CIDR:        types.StringValue(b.CIDR),
		Name:        types.StringValue(b.Name),
		Description: types.StringValue(b.Description),
		Origin:      types.StringValue(b.Origin),
		Family:      types.Int64Value(int64(b.Family)),
		Version:     types.Int64Value(int64(b.Version)),
	}
	if b.ParentBlockID != nil {
		m.ParentBlockID = types.StringValue(*b.ParentBlockID)
	} else {
		m.ParentBlockID = types.StringNull()
	}
	return m
}
