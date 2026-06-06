package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                = (*poolResource)(nil)
	_ resource.ResourceWithConfigure   = (*poolResource)(nil)
	_ resource.ResourceWithImportState = (*poolResource)(nil)
)

func NewPoolResource() resource.Resource { return &poolResource{} }

type poolResource struct {
	client *client.Client
}

type poolResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	HealthMonitorID types.String `tfsdk:"health_monitor_id"`
	SelectionMethod types.String `tfsdk:"selection_method"`
	MemberCount     types.Int64  `tfsdk:"member_count"`
	HealthScore     types.Int64  `tfsdk:"health_score"`
}

func (r *poolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (r *poolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A GSLB pool of endpoints with a selection algorithm.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Pool name, unique within the tenant.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description shown in the console.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"health_monitor_id": schema.StringAttribute{
				MarkdownDescription: "ID of the health monitor used to probe pool members.",
				Required:            true,
			},
			"selection_method": schema.StringAttribute{
				MarkdownDescription: "Load-balancing algorithm. One of `weighted`, `active-passive`, `round-robin`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("weighted"),
				Validators: []validator.String{
					stringvalidator.OneOf("weighted", "active-passive", "round-robin"),
				},
			},
			"member_count": schema.Int64Attribute{
				MarkdownDescription: "Number of members in the pool. Computed.",
				Computed:            true,
			},
			"health_score": schema.Int64Attribute{
				MarkdownDescription: "Pool health score, 0 to 100. Computed from member up/total.",
				Computed:            true,
			},
		},
	}
}

func (r *poolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *poolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan poolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.Pool{
		Name:            plan.Name.ValueString(),
		Description:     plan.Description.ValueString(),
		HealthMonitorID: plan.HealthMonitorID.ValueString(),
		SelectionMethod: plan.SelectionMethod.ValueString(),
	}
	got, err := r.client.CreatePool(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("create pool", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIPool(got))...)
}

func (r *poolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state poolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetPool(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read pool", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIPool(got))...)
}

func (r *poolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan poolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	desc := plan.Description.ValueString()
	mon := plan.HealthMonitorID.ValueString()
	sel := plan.SelectionMethod.ValueString()
	got, err := r.client.UpdatePool(ctx, plan.ID.ValueString(), client.PoolUpdate{
		Name:            &name,
		Description:     &desc,
		HealthMonitorID: &mon,
		SelectionMethod: &sel,
	})
	if err != nil {
		resp.Diagnostics.AddError("update pool", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIPool(got))...)
}

func (r *poolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state poolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePool(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete pool", err.Error())
		return
	}
}

func (r *poolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func fromAPIPool(p *client.Pool) poolResourceModel {
	return poolResourceModel{
		ID:              types.StringValue(p.ID),
		Name:            types.StringValue(p.Name),
		Description:     types.StringValue(p.Description),
		HealthMonitorID: types.StringValue(p.HealthMonitorID),
		SelectionMethod: types.StringValue(p.SelectionMethod),
		MemberCount:     types.Int64Value(int64(p.MemberCount)),
		HealthScore:     types.Int64Value(int64(p.HealthScore)),
	}
}
