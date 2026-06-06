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
	_ resource.Resource                = (*endpointResource)(nil)
	_ resource.ResourceWithConfigure   = (*endpointResource)(nil)
	_ resource.ResourceWithImportState = (*endpointResource)(nil)
)

func NewEndpointResource() resource.Resource { return &endpointResource{} }

type endpointResource struct {
	client *client.Client
}

type endpointResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Kind            types.String `tfsdk:"kind"`
	Value           types.String `tfsdk:"value"`
	Host            types.String `tfsdk:"host"`
	Port            types.Int64  `tfsdk:"port"`
	HealthMonitorID types.String `tfsdk:"health_monitor_id"`
}

func (r *endpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (r *endpointResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A health-checked endpoint that pool members reference.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable endpoint name.",
				Required:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Probe kind: `http`, `https`, or `tcp`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("http", "https", "tcp"),
				},
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "DNS answer value returned when this endpoint is selected. Typically an IP address or target hostname.",
				Required:            true,
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "Hostname or IP used by the health probe. Required when health_monitor_id is set.",
				Optional:            true,
				Computed:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "Probe port. 0 (or omitted) means use the monitor's default (443 for HTTPS, 80 for HTTP).",
				Optional:            true,
				Computed:            true,
			},
			"health_monitor_id": schema.StringAttribute{
				MarkdownDescription: "ID of the health monitor used to probe this endpoint. Recommended over inline kind+target.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *endpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *endpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := planToEndpoint(plan)
	got, err := r.client.CreateEndpoint(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("create endpoint", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIEndpoint(got))...)
}

func (r *endpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetEndpoint(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read endpoint", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIEndpoint(got))...)
}

func (r *endpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan endpointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.UpdateEndpoint(ctx, plan.ID.ValueString(), planToEndpoint(plan))
	if err != nil {
		resp.Diagnostics.AddError("update endpoint", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIEndpoint(got))...)
}

// planToEndpoint maps the Terraform resource model to the API request
// shape. Target is fixed up here: the server requires it even when a
// health monitor is configured, but Terraform users typically only
// care about value (the DNS answer) and host/port (the probe knob).
// Fall back to value (or host if value is empty) so the server stays
// happy without forcing users to set target explicitly.
func planToEndpoint(m endpointResourceModel) client.Endpoint {
	value := m.Value.ValueString()
	host := m.Host.ValueString()
	target := value
	if target == "" {
		target = host
	}
	return client.Endpoint{
		Name:            m.Name.ValueString(),
		Kind:            m.Kind.ValueString(),
		Target:          target,
		Value:           value,
		Host:            host,
		Port:            int(m.Port.ValueInt64()),
		HealthMonitorID: m.HealthMonitorID.ValueString(),
	}
}

func (r *endpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state endpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEndpoint(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete endpoint", err.Error())
		return
	}
}

func (r *endpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func fromAPIEndpoint(e *client.Endpoint) endpointResourceModel {
	m := endpointResourceModel{
		ID:    types.StringValue(e.ID),
		Name:  types.StringValue(e.Name),
		Value: types.StringValue(e.Value),
	}
	if e.Kind != "" {
		m.Kind = types.StringValue(e.Kind)
	} else {
		m.Kind = types.StringNull()
	}
	if e.Host != "" {
		m.Host = types.StringValue(e.Host)
	} else {
		m.Host = types.StringNull()
	}
	m.Port = types.Int64Value(int64(e.Port))
	if e.HealthMonitorID != "" {
		m.HealthMonitorID = types.StringValue(e.HealthMonitorID)
	} else {
		m.HealthMonitorID = types.StringNull()
	}
	return m
}
