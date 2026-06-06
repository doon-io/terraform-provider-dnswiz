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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                = (*healthMonitorResource)(nil)
	_ resource.ResourceWithConfigure   = (*healthMonitorResource)(nil)
	_ resource.ResourceWithImportState = (*healthMonitorResource)(nil)
)

func NewHealthMonitorResource() resource.Resource { return &healthMonitorResource{} }

type healthMonitorResource struct {
	client *client.Client
}

type healthMonitorResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Kind            types.String `tfsdk:"kind"`
	Path            types.String `tfsdk:"path"`
	ExpectedStatus  types.Int64  `tfsdk:"expected_status"`
	IntervalSeconds types.Int64  `tfsdk:"interval_seconds"`
	TimeoutSeconds  types.Int64  `tfsdk:"timeout_seconds"`
	HealthyAfter    types.Int64  `tfsdk:"healthy_after"`
	UnhealthyAfter  types.Int64  `tfsdk:"unhealthy_after"`
	IsPreset        types.Bool   `tfsdk:"is_preset"`
}

func (r *healthMonitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_health_monitor"
}

func (r *healthMonitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A reusable health-check probe definition. Multiple endpoints and pools can share one monitor.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Monitor name, unique within the tenant.",
				Required:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Probe kind. One of `http`, `https`, `tcp`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("http", "https", "tcp"),
				},
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "Path requested by HTTP/HTTPS probes (e.g. `/healthz`). Ignored for TCP.",
				Optional:            true,
				Computed:            true,
			},
			"expected_status": schema.Int64Attribute{
				MarkdownDescription: "HTTP response code expected for a healthy probe. Defaults to 200.",
				Optional:            true,
				Computed:            true,
			},
			"interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "How often probes run, in seconds.",
				Optional:            true,
				Computed:            true,
			},
			"timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Probe timeout in seconds.",
				Optional:            true,
				Computed:            true,
			},
			"healthy_after": schema.Int64Attribute{
				MarkdownDescription: "Consecutive successful probes required to flip an endpoint to up.",
				Optional:            true,
				Computed:            true,
			},
			"unhealthy_after": schema.Int64Attribute{
				MarkdownDescription: "Consecutive failed probes required to flip an endpoint to down.",
				Optional:            true,
				Computed:            true,
			},
			"is_preset": schema.BoolAttribute{
				MarkdownDescription: "True for monitors owned by dnswiz. Presets cannot be modified or deleted.",
				Computed:            true,
			},
		},
	}
}

func (r *healthMonitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *healthMonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan healthMonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.CreateHealthMonitor(ctx, planToHM(plan))
	if err != nil {
		resp.Diagnostics.AddError("create health monitor", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIHM(got))...)
}

func (r *healthMonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state healthMonitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetHealthMonitor(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read health monitor", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIHM(got))...)
}

func (r *healthMonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan healthMonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.UpdateHealthMonitor(ctx, plan.ID.ValueString(), planToHM(plan))
	if err != nil {
		resp.Diagnostics.AddError("update health monitor", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIHM(got))...)
}

func (r *healthMonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state healthMonitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteHealthMonitor(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete health monitor", err.Error())
		return
	}
}

func (r *healthMonitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func planToHM(m healthMonitorResourceModel) client.HealthMonitor {
	return client.HealthMonitor{
		Name:            m.Name.ValueString(),
		Kind:            m.Kind.ValueString(),
		Path:            m.Path.ValueString(),
		ExpectedStatus:  int(m.ExpectedStatus.ValueInt64()),
		IntervalSeconds: int(m.IntervalSeconds.ValueInt64()),
		TimeoutSeconds:  int(m.TimeoutSeconds.ValueInt64()),
		HealthyAfter:    int(m.HealthyAfter.ValueInt64()),
		UnhealthyAfter:  int(m.UnhealthyAfter.ValueInt64()),
	}
}

func fromAPIHM(m *client.HealthMonitor) healthMonitorResourceModel {
	return healthMonitorResourceModel{
		ID:              types.StringValue(m.ID),
		Name:            types.StringValue(m.Name),
		Kind:            types.StringValue(m.Kind),
		Path:            types.StringValue(m.Path),
		ExpectedStatus:  types.Int64Value(int64(m.ExpectedStatus)),
		IntervalSeconds: types.Int64Value(int64(m.IntervalSeconds)),
		TimeoutSeconds:  types.Int64Value(int64(m.TimeoutSeconds)),
		HealthyAfter:    types.Int64Value(int64(m.HealthyAfter)),
		UnhealthyAfter:  types.Int64Value(int64(m.UnhealthyAfter)),
		IsPreset:        types.BoolValue(m.IsPreset),
	}
}
