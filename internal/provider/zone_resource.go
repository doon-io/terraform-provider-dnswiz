package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                = (*zoneResource)(nil)
	_ resource.ResourceWithConfigure   = (*zoneResource)(nil)
	_ resource.ResourceWithImportState = (*zoneResource)(nil)
)

func NewZoneResource() resource.Resource { return &zoneResource{} }

type zoneResource struct {
	client *client.Client
}

type zoneResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	DefaultTTL  types.Int64  `tfsdk:"default_ttl"`
	RName       types.String `tfsdk:"rname"`
	NegativeTTL types.Int64  `tfsdk:"negative_ttl"`
}

func (r *zoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (r *zoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A DNS zone managed by dnswiz.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Zone ID assigned by dnswiz.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Apex name of the zone (e.g. `example.com`). Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"default_ttl": schema.Int64Attribute{
				MarkdownDescription: "Default TTL (in seconds) for records in this zone that don't set their own TTL.",
				Optional:            true,
				Computed:            true,
			},
			"rname": schema.StringAttribute{
				MarkdownDescription: "RNAME (responsible-person email) used in the SOA record. Defaults to the tenant-wide setting.",
				Optional:            true,
				Computed:            true,
			},
			"negative_ttl": schema.Int64Attribute{
				MarkdownDescription: "Negative-cache TTL used in the SOA record minimum field.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *zoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *zoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan zoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := r.client.CreateZone(ctx, toClientZone(plan))
	if err != nil {
		resp.Diagnostics.AddError("create zone", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromClientZone(out))...)
}

func (r *zoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state zoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.GetZone(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			// Zone was deleted out of band. Drop it from state
			// so the next plan recreates it.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read zone", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromClientZone(out))...)
}

func (r *zoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan zoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.UpdateZone(ctx, plan.ID.ValueString(), toClientZone(plan))
	if err != nil {
		resp.Diagnostics.AddError("update zone", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromClientZone(out))...)
}

func (r *zoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state zoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteZone(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete zone", err.Error())
		return
	}
}

func (r *zoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func toClientZone(m zoneResourceModel) client.Zone {
	z := client.Zone{
		ID:   m.ID.ValueString(),
		Name: m.Name.ValueString(),
	}
	if !m.DefaultTTL.IsNull() && !m.DefaultTTL.IsUnknown() {
		z.DefaultTTL = int(m.DefaultTTL.ValueInt64())
	}
	if !m.RName.IsNull() && !m.RName.IsUnknown() {
		z.RName = m.RName.ValueString()
	}
	if !m.NegativeTTL.IsNull() && !m.NegativeTTL.IsUnknown() {
		z.NegativeTTL = int(m.NegativeTTL.ValueInt64())
	}
	return z
}

func fromClientZone(z *client.Zone) zoneResourceModel {
	return zoneResourceModel{
		ID:          types.StringValue(z.ID),
		Name:        types.StringValue(z.Name),
		DefaultTTL:  types.Int64Value(int64(z.DefaultTTL)),
		RName:       types.StringValue(z.RName),
		NegativeTTL: types.Int64Value(int64(z.NegativeTTL)),
	}
}
