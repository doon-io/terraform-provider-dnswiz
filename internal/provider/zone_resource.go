package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
	Active      types.Bool   `tfsdk:"active"`
	DefaultTTL  types.Int64  `tfsdk:"default_ttl"`
	SOARName    types.String `tfsdk:"soa_rname"`
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
				MarkdownDescription: "Apex name of the zone, for example `example.com`. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the zone is served. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"default_ttl": schema.Int64Attribute{
				MarkdownDescription: "Default TTL in seconds for records in this zone that do not set their own. Omit to inherit the tenant default.",
				Optional:            true,
				Computed:            true,
			},
			"soa_rname": schema.StringAttribute{
				MarkdownDescription: "Responsible-person email used in the SOA record. Omit to inherit the tenant default.",
				Optional:            true,
				Computed:            true,
			},
			"negative_ttl": schema.Int64Attribute{
				MarkdownDescription: "Negative-cache TTL used in the SOA minimum field. Omit to inherit the tenant default.",
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

	created, err := r.client.CreateZone(ctx, client.ZoneCreate{Name: plan.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("create zone", err.Error())
		return
	}

	// If the user set optional fields on create, apply them in a follow-up
	// PATCH since the create endpoint only takes name.
	if needsZonePatch(plan) {
		updated, err := r.client.UpdateZone(ctx, created.ID, zoneUpdateFromPlan(plan))
		if err != nil {
			resp.Diagnostics.AddError("apply initial zone settings", err.Error())
			return
		}
		created = updated
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIZone(created))...)
}

func (r *zoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state zoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetZone(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			// Zone was deleted out of band. Drop it from state so the
			// next plan recreates it.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read zone", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIZone(got))...)
}

func (r *zoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan zoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.UpdateZone(ctx, plan.ID.ValueString(), zoneUpdateFromPlan(plan))
	if err != nil {
		resp.Diagnostics.AddError("update zone", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIZone(got))...)
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

func needsZonePatch(m zoneResourceModel) bool {
	return !m.Active.IsNull() || !m.DefaultTTL.IsNull() || !m.SOARName.IsNull() || !m.NegativeTTL.IsNull()
}

func zoneUpdateFromPlan(m zoneResourceModel) client.ZoneUpdate {
	var u client.ZoneUpdate
	if !m.Active.IsNull() && !m.Active.IsUnknown() {
		v := m.Active.ValueBool()
		u.Active = &v
	}
	if !m.DefaultTTL.IsNull() && !m.DefaultTTL.IsUnknown() {
		v := int(m.DefaultTTL.ValueInt64())
		u.DefaultTTL = &v
	}
	if !m.SOARName.IsNull() && !m.SOARName.IsUnknown() {
		v := m.SOARName.ValueString()
		u.SOARName = &v
	}
	if !m.NegativeTTL.IsNull() && !m.NegativeTTL.IsUnknown() {
		v := int(m.NegativeTTL.ValueInt64())
		u.NegativeTTL = &v
	}
	return u
}

func fromAPIZone(z *client.Zone) zoneResourceModel {
	m := zoneResourceModel{
		ID:     types.StringValue(z.ID),
		Name:   types.StringValue(z.Name),
		Active: types.BoolValue(z.Active),
	}
	if z.DefaultTTL != nil {
		m.DefaultTTL = types.Int64Value(int64(*z.DefaultTTL))
	} else {
		m.DefaultTTL = types.Int64Null()
	}
	if z.SOARName != nil {
		m.SOARName = types.StringValue(*z.SOARName)
	} else {
		m.SOARName = types.StringNull()
	}
	if z.NegativeTTL != nil {
		m.NegativeTTL = types.Int64Value(int64(*z.NegativeTTL))
	} else {
		m.NegativeTTL = types.Int64Null()
	}
	return m
}
