package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	_ resource.Resource                = (*zonePolicyResource)(nil)
	_ resource.ResourceWithConfigure   = (*zonePolicyResource)(nil)
	_ resource.ResourceWithImportState = (*zonePolicyResource)(nil)
)

func NewZonePolicyResource() resource.Resource { return &zonePolicyResource{} }

type zonePolicyResource struct {
	client *client.Client
}

type zonePolicyResourceModel struct {
	ID         types.String `tfsdk:"id"`
	ZoneID     types.String `tfsdk:"zone_id"`
	Kind       types.String `tfsdk:"kind"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	ConfigJSON types.String `tfsdk:"config_json"`
}

func (r *zonePolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone_policy"
}

func (r *zonePolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A security policy attached to a zone. Today the server supports `hijack_monitor`, `query_firewall`, `change_approval`, `ttl_guardrail`, `caa_enforcement`, and `api_geo_lock`. The config shape is kind-specific; pass it as a JSON-encoded string.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite ID `<zone_id>/<kind>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_id": schema.StringAttribute{
				MarkdownDescription: "Zone the policy applies to. Changing this forces a new policy.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Policy kind. Changing this forces a new policy.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("hijack_monitor", "query_firewall", "change_approval", "ttl_guardrail", "caa_enforcement", "api_geo_lock"),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the policy is active for the zone.",
				Required:            true,
			},
			"config_json": schema.StringAttribute{
				MarkdownDescription: "Kind-specific configuration as a JSON-encoded string. Use Terraform's `jsonencode()` to build it. Omit or set to `{}` for kinds without configurable options. See the dnswiz docs for per-kind schemas.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *zonePolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *zonePolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan zonePolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.applyPolicy(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("create zone policy", err.Error())
		return
	}
	out := fromAPIZonePolicy(plan.ZoneID.ValueString(), got)
	// Preserve the user's JSON string verbatim if it parses to the same
	// data as the server returned. Server can reorder keys, which would
	// otherwise look like drift even though the policy is identical.
	out.ConfigJSON = preferUserJSON(plan.ConfigJSON, out.ConfigJSON)
	resp.Diagnostics.Append(resp.State.Set(ctx, out)...)
}

func (r *zonePolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state zonePolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policies, err := r.client.ListZonePolicies(ctx, state.ZoneID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read zone policies", err.Error())
		return
	}
	kind := state.Kind.ValueString()
	for i := range policies {
		if policies[i].Kind == kind {
			out := fromAPIZonePolicy(state.ZoneID.ValueString(), &policies[i])
			out.ConfigJSON = preferUserJSON(state.ConfigJSON, out.ConfigJSON)
			resp.Diagnostics.Append(resp.State.Set(ctx, out)...)
			return
		}
	}
	// Server omits rows for kinds that have never been touched. Treat as
	// disabled + empty config so Terraform plans an apply on next run if
	// the user wants the policy enabled.
	resp.Diagnostics.Append(resp.State.Set(ctx, zonePolicyResourceModel{
		ID:         state.ID,
		ZoneID:     state.ZoneID,
		Kind:       state.Kind,
		Enabled:    types.BoolValue(false),
		ConfigJSON: types.StringValue("{}"),
	})...)
}

func (r *zonePolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan zonePolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.applyPolicy(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("update zone policy", err.Error())
		return
	}
	out := fromAPIZonePolicy(plan.ZoneID.ValueString(), got)
	out.ConfigJSON = preferUserJSON(plan.ConfigJSON, out.ConfigJSON)
	resp.Diagnostics.Append(resp.State.Set(ctx, out)...)
}

func (r *zonePolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state zonePolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The server has no DELETE for policies; "deleting" means disabling
	// the row. The row stays around so future PATCHes can re-enable it
	// without losing config history.
	disabled := false
	if _, err := r.client.PatchZonePolicy(ctx, state.ZoneID.ValueString(), state.Kind.ValueString(), client.ZonePolicyUpdate{
		Enabled: &disabled,
	}); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("disable zone policy", err.Error())
		return
	}
}

func (r *zonePolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("invalid import id", "expected zone_id/kind")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kind"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *zonePolicyResource) applyPolicy(ctx context.Context, plan zonePolicyResourceModel) (*client.ZonePolicy, error) {
	enabled := plan.Enabled.ValueBool()
	up := client.ZonePolicyUpdate{Enabled: &enabled}
	if !plan.ConfigJSON.IsNull() && !plan.ConfigJSON.IsUnknown() && plan.ConfigJSON.ValueString() != "" {
		raw := json.RawMessage(plan.ConfigJSON.ValueString())
		if !json.Valid(raw) {
			return nil, fmt.Errorf("config_json is not valid JSON")
		}
		up.Config = &raw
	}
	return r.client.PatchZonePolicy(ctx, plan.ZoneID.ValueString(), plan.Kind.ValueString(), up)
}

// preferUserJSON returns the user's JSON string verbatim if it parses
// to the same data as the server's. JSON object key order is not
// semantically meaningful, but Terraform compares the raw strings, so
// the server reordering keys would otherwise look like state drift.
func preferUserJSON(user, server types.String) types.String {
	if user.IsNull() || user.IsUnknown() {
		return server
	}
	if user.ValueString() == server.ValueString() {
		return user
	}
	var a, b any
	if json.Unmarshal([]byte(user.ValueString()), &a) != nil {
		return server
	}
	if json.Unmarshal([]byte(server.ValueString()), &b) != nil {
		return server
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) == string(jb) {
		return user
	}
	return server
}

func fromAPIZonePolicy(zoneID string, p *client.ZonePolicy) zonePolicyResourceModel {
	cfg := "{}"
	if len(p.Config) > 0 && string(p.Config) != "null" {
		cfg = string(p.Config)
	}
	return zonePolicyResourceModel{
		ID:         types.StringValue(zoneID + "/" + p.Kind),
		ZoneID:     types.StringValue(zoneID),
		Kind:       types.StringValue(p.Kind),
		Enabled:    types.BoolValue(p.Enabled),
		ConfigJSON: types.StringValue(cfg),
	}
}
