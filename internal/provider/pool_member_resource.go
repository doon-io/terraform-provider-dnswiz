package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                = (*poolMemberResource)(nil)
	_ resource.ResourceWithConfigure   = (*poolMemberResource)(nil)
	_ resource.ResourceWithImportState = (*poolMemberResource)(nil)
)

func NewPoolMemberResource() resource.Resource { return &poolMemberResource{} }

type poolMemberResource struct {
	client *client.Client
}

type poolMemberResourceModel struct {
	ID         types.String `tfsdk:"id"`
	PoolID     types.String `tfsdk:"pool_id"`
	EndpointID types.String `tfsdk:"endpoint_id"`
	Weight     types.Int64  `tfsdk:"weight"`
	Priority   types.Int64  `tfsdk:"priority"`
	Enabled    types.Bool   `tfsdk:"enabled"`
}

func (r *poolMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool_member"
}

func (r *poolMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Binds an endpoint into a pool with a per-pool weight and priority.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite ID `<pool_id>/<member_id>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"pool_id": schema.StringAttribute{
				MarkdownDescription: "Parent pool ID. Changing this forces a new membership.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoint_id": schema.StringAttribute{
				MarkdownDescription: "ID of the endpoint to put in the pool. Changing this forces a new membership.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"weight": schema.Int64Attribute{
				MarkdownDescription: "Relative weight inside the pool. Must be 1..10000. Defaults to 100.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(100),
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "Priority for active-passive pools. Lower wins. Must be 1..1000. Defaults to 1.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the engine considers this member when selecting an answer. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *poolMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *poolMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan poolMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.client.AddPoolMember(ctx, plan.PoolID.ValueString(), client.PoolMemberCreate{
		EndpointID: plan.EndpointID.ValueString(),
		Weight:     int(plan.Weight.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError("add pool member", err.Error())
		return
	}

	// Priority and enabled aren't accepted on the create endpoint, so
	// apply them in follow-up calls if the user set non-default values.
	if plan.Priority.ValueInt64() != 1 {
		prio := int(plan.Priority.ValueInt64())
		w := int(plan.Weight.ValueInt64())
		updated, err := r.client.UpdatePoolMember(ctx, plan.PoolID.ValueString(), member.ID, client.PoolMemberUpdate{
			Weight:   &w,
			Priority: &prio,
		})
		if err != nil {
			resp.Diagnostics.AddError("set pool member priority", err.Error())
			return
		}
		member = updated
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && !plan.Enabled.ValueBool() {
		if err := r.client.SetPoolMemberEnabled(ctx, plan.PoolID.ValueString(), member.ID, false); err != nil {
			resp.Diagnostics.AddError("disable pool member", err.Error())
			return
		}
		member.Enabled = false
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIPoolMember(member, plan.PoolID.ValueString()))...)
}

func (r *poolMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state poolMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	poolID, memberID, ok := splitMemberID(state.ID.ValueString(), state.PoolID.ValueString())
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}
	got, err := r.client.GetPoolMember(ctx, poolID, memberID)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read pool member", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIPoolMember(got, poolID))...)
}

func (r *poolMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state poolMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	poolID, memberID, ok := splitMemberID(state.ID.ValueString(), state.PoolID.ValueString())
	if !ok {
		resp.Diagnostics.AddError("invalid id", "could not parse pool member id from state")
		return
	}

	w := int(plan.Weight.ValueInt64())
	prio := int(plan.Priority.ValueInt64())
	got, err := r.client.UpdatePoolMember(ctx, poolID, memberID, client.PoolMemberUpdate{
		Weight:   &w,
		Priority: &prio,
	})
	if err != nil {
		resp.Diagnostics.AddError("update pool member", err.Error())
		return
	}
	if plan.Enabled.ValueBool() != got.Enabled {
		if err := r.client.SetPoolMemberEnabled(ctx, poolID, memberID, plan.Enabled.ValueBool()); err != nil {
			resp.Diagnostics.AddError("toggle pool member", err.Error())
			return
		}
		got.Enabled = plan.Enabled.ValueBool()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIPoolMember(got, poolID))...)
}

func (r *poolMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state poolMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	poolID, memberID, ok := splitMemberID(state.ID.ValueString(), state.PoolID.ValueString())
	if !ok {
		return
	}
	if err := r.client.RemovePoolMember(ctx, poolID, memberID); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("remove pool member", err.Error())
		return
	}
}

func (r *poolMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expect "pool_id/member_id" format on import.
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("invalid import id", "expected pool_id/member_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("pool_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func splitMemberID(compositeID, poolID string) (string, string, bool) {
	parts := strings.SplitN(compositeID, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	if poolID != "" && parts[0] != poolID {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func fromAPIPoolMember(m *client.PoolMember, poolID string) poolMemberResourceModel {
	return poolMemberResourceModel{
		ID:         types.StringValue(poolID + "/" + m.ID),
		PoolID:     types.StringValue(poolID),
		EndpointID: types.StringValue(m.EndpointID),
		Weight:     types.Int64Value(int64(m.Weight)),
		Priority:   types.Int64Value(int64(m.Priority)),
		Enabled:    types.BoolValue(m.Enabled),
	}
}
