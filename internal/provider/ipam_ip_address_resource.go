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
	_ resource.Resource                = (*ipamIPResource)(nil)
	_ resource.ResourceWithConfigure   = (*ipamIPResource)(nil)
	_ resource.ResourceWithImportState = (*ipamIPResource)(nil)
)

func NewIPAMIPAddressResource() resource.Resource { return &ipamIPResource{} }

type ipamIPResource struct {
	client *client.Client
}

type ipamIPResourceModel struct {
	ID          types.String `tfsdk:"id"`
	NetworkID   types.String `tfsdk:"network_id"`
	Address     types.String `tfsdk:"address"`
	Hostname    types.String `tfsdk:"hostname"`
	Description types.String `tfsdk:"description"`
	Status      types.String `tfsdk:"status"`
	Tags        types.Set    `tfsdk:"tags"`
	Family      types.Int64  `tfsdk:"family"`
	Version     types.Int64  `tfsdk:"version"`
}

func (r *ipamIPResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipam_ip_address"
}

func (r *ipamIPResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An IPAM address inside a network. Set `address` explicitly, or omit it to take the next free host in `network_id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Address ID assigned by dnswiz.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "Network the address belongs to. Forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"address": schema.StringAttribute{
				MarkdownDescription: "The host address, e.g. `10.1.5.10`. Omit to auto-allocate the next free host. Changing an explicit value forces a new resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "Optional hostname for the address.",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Address status: `active`, `reserved`, `deprecated`, `dhcp`, `slaac`, or `pending`. Defaults to `active`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("active"),
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Tag names to attach to the address. Missing tags are created.",
				Optional:            true,
				ElementType:         types.StringType,
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

func (r *ipamIPResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ipamIPResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ipamIPResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ip *client.IPAMAddress
	var err error

	if !plan.Address.IsNull() && !plan.Address.IsUnknown() {
		ip, err = r.client.CreateIP(ctx, client.IPAMAddressCreate{
			NetworkID:   plan.NetworkID.ValueString(),
			Addr:        plan.Address.ValueString(),
			Status:      plan.Status.ValueString(),
			Hostname:    optString(plan.Hostname),
			Description: plan.Description.ValueString(),
		})
	} else {
		ip, err = r.client.AllocateIP(ctx, plan.NetworkID.ValueString())
		if err == nil && (plan.Status.ValueString() != "active" || optString(plan.Hostname) != nil || plan.Description.ValueString() != "") {
			ip, err = r.client.UpdateIP(ctx, ip.ID, ip.Version, client.IPAMAddressUpdate{
				Status:      plan.Status.ValueString(),
				Hostname:    optString(plan.Hostname),
				Description: plan.Description.ValueString(),
			})
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("create ipam address", err.Error())
		return
	}

	if d := assignResourceTags(ctx, r.client, "ip_address", ip.ID, plan.Tags); d != nil {
		resp.Diagnostics.Append(d...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, ipModelFrom(ip, plan.Tags))...)
}

func (r *ipamIPResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ipamIPResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetIP(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read ipam address", err.Error())
		return
	}
	names, err := r.client.TagNamesForResource(ctx, "ip_address", got.ID)
	if err != nil {
		resp.Diagnostics.AddError("read address tags", err.Error())
		return
	}
	tagSet, d := tagsToSet(ctx, state.Tags, names)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, ipModelFrom(got, tagSet))...)
}

func (r *ipamIPResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ipamIPResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cur, err := r.client.GetIP(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("read address for update", err.Error())
		return
	}
	got, err := r.client.UpdateIP(ctx, plan.ID.ValueString(), cur.Version, client.IPAMAddressUpdate{
		Status:      plan.Status.ValueString(),
		Hostname:    optString(plan.Hostname),
		Description: plan.Description.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("update ipam address", err.Error())
		return
	}
	if d := assignResourceTags(ctx, r.client, "ip_address", got.ID, plan.Tags); d != nil {
		resp.Diagnostics.Append(d...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, ipModelFrom(got, plan.Tags))...)
}

func (r *ipamIPResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ipamIPResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteIP(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete ipam address", err.Error())
		return
	}
}

func (r *ipamIPResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func ipModelFrom(ip *client.IPAMAddress, tagSet types.Set) ipamIPResourceModel {
	m := ipamIPResourceModel{
		ID:          types.StringValue(ip.ID),
		NetworkID:   types.StringValue(ip.NetworkID),
		Address:     types.StringValue(ip.Addr),
		Description: types.StringValue(ip.Description),
		Status:      types.StringValue(ip.Status),
		Family:      types.Int64Value(int64(ip.Family)),
		Version:     types.Int64Value(int64(ip.Version)),
		Tags:        tagSet,
	}
	if ip.Hostname != nil {
		m.Hostname = types.StringValue(*ip.Hostname)
	} else {
		m.Hostname = types.StringNull()
	}
	return m
}
