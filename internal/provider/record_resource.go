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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                = (*recordResource)(nil)
	_ resource.ResourceWithConfigure   = (*recordResource)(nil)
	_ resource.ResourceWithImportState = (*recordResource)(nil)
)

func NewRecordResource() resource.Resource { return &recordResource{} }

type recordResource struct {
	client *client.Client
}

type recordResourceModel struct {
	ID         types.String `tfsdk:"id"`
	ZoneID     types.String `tfsdk:"zone_id"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	TTL        types.Int64  `tfsdk:"ttl"`
	TTLInherit types.Bool   `tfsdk:"ttl_inherit"`
	Active     types.Bool   `tfsdk:"active"`
	Comment    types.String `tfsdk:"comment"`

	// Type-specific. Set the ones that apply to your record type.
	Value    types.String `tfsdk:"value"`
	Priority types.Int64  `tfsdk:"priority"`
	Weight   types.Int64  `tfsdk:"weight"`
	Port     types.Int64  `tfsdk:"port"`
	Tag      types.String `tfsdk:"tag"`
	Flags    types.Int64  `tfsdk:"flags"`
	PoolID   types.String `tfsdk:"pool_id"`
}

func (r *recordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *recordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A DNS record inside a dnswiz zone. Supported types: A, AAAA, CNAME, NS, PTR, TXT, ANAME, MX, SRV, CAA, POOL.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_id": schema.StringAttribute{
				MarkdownDescription: "ID of the zone the record belongs to. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Record owner relative to the zone. Use `@` for the apex.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Record type. One of A, AAAA, CNAME, NS, PTR, TXT, ANAME, MX, SRV, CAA, POOL. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("A", "AAAA", "CNAME", "NS", "PTR", "TXT", "ANAME", "MX", "SRV", "CAA", "POOL"),
				},
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "Record TTL in seconds. Omit to inherit the zone default.",
				Optional:            true,
				Computed:            true,
			},
			"ttl_inherit": schema.BoolAttribute{
				MarkdownDescription: "If true, the record's TTL tracks zone.default_ttl going forward. If false, the TTL is frozen at the value set at create or last update.",
				Optional:            true,
				Computed:            true,
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the record is served. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"comment": schema.StringAttribute{
				MarkdownDescription: "Free-text annotation visible in the dnswiz console.",
				Optional:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "Type-dependent value. For A and AAAA the IP address. For CNAME, NS, PTR, ANAME the target name. For TXT the string content. For MX and SRV the target host. For CAA the property value.",
				Optional:            true,
				Computed:            true,
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "MX preference or SRV priority.",
				Optional:            true,
				Computed:            true,
			},
			"weight": schema.Int64Attribute{
				MarkdownDescription: "SRV weight.",
				Optional:            true,
				Computed:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "SRV port.",
				Optional:            true,
				Computed:            true,
			},
			"tag": schema.StringAttribute{
				MarkdownDescription: "CAA property tag: `issue`, `issuewild`, or `iodef`.",
				Optional:            true,
				Computed:            true,
			},
			"flags": schema.Int64Attribute{
				MarkdownDescription: "CAA flags byte. Typically 0 or 128 (critical).",
				Optional:            true,
				Computed:            true,
			},
			"pool_id": schema.StringAttribute{
				MarkdownDescription: "Pool ID for POOL records.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *recordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *recordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan recordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, err := encodeRecordData(plan.Type.ValueString(), plan)
	if err != nil {
		resp.Diagnostics.AddError("encode record data", err.Error())
		return
	}

	in := client.Record{
		Name:    plan.Name.ValueString(),
		Type:    plan.Type.ValueString(),
		TTL:     int(plan.TTL.ValueInt64()),
		Data:    data,
		Comment: plan.Comment.ValueString(),
	}
	if !plan.TTLInherit.IsNull() && !plan.TTLInherit.IsUnknown() {
		in.TTLInherit = plan.TTLInherit.ValueBool()
	}

	created, err := r.client.CreateRecord(ctx, plan.ZoneID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("create record", err.Error())
		return
	}

	model := fromAPIRecord(created, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *recordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state recordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetRecord(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read record", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIRecord(got, state))...)
}

func (r *recordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan recordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, err := encodeRecordData(plan.Type.ValueString(), plan)
	if err != nil {
		resp.Diagnostics.AddError("encode record data", err.Error())
		return
	}

	name := plan.Name.ValueString()
	ttl := int(plan.TTL.ValueInt64())
	active := plan.Active.ValueBool()
	comment := plan.Comment.ValueString()

	up := client.RecordUpdate{
		Name:    &name,
		TTL:     &ttl,
		Data:    data,
		Active:  &active,
		Comment: &comment,
	}
	if !plan.TTLInherit.IsNull() && !plan.TTLInherit.IsUnknown() {
		v := plan.TTLInherit.ValueBool()
		up.TTLInherit = &v
	}

	got, err := r.client.UpdateRecord(ctx, plan.ID.ValueString(), up)
	if err != nil {
		resp.Diagnostics.AddError("update record", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fromAPIRecord(got, plan))...)
}

func (r *recordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state recordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteRecord(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete record", err.Error())
		return
	}
}

func (r *recordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func fromAPIRecord(rec *client.Record, _ recordResourceModel) recordResourceModel {
	// Start every type-specific attribute as null. decodeRecordData
	// fills in just the ones that apply to this record's type. Anything
	// not applicable stays null, which lets users omit it from HCL
	// without terraform thinking there's drift.
	m := recordResourceModel{
		ID:         types.StringValue(rec.ID),
		ZoneID:     types.StringValue(rec.ZoneID),
		Name:       types.StringValue(rec.Name),
		Type:       types.StringValue(rec.Type),
		TTL:        types.Int64Value(int64(rec.TTL)),
		TTLInherit: types.BoolValue(rec.TTLInherit),
		Active:     types.BoolValue(rec.Active),
		Value:      types.StringNull(),
		Priority:   types.Int64Null(),
		Weight:     types.Int64Null(),
		Port:       types.Int64Null(),
		Tag:        types.StringNull(),
		Flags:      types.Int64Null(),
		PoolID:     types.StringNull(),
	}
	if rec.Comment != "" {
		m.Comment = types.StringValue(rec.Comment)
	} else {
		m.Comment = types.StringNull()
	}
	_ = decodeRecordData(rec.Type, rec.Data, &m)
	return m
}
