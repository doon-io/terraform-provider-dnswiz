package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

// One file because every lookup-by-name data source is the same shape:
// take a name, list the collection, find the row. The variation is in
// the result attributes, which we keep small (id + a couple of useful
// fields) since users typically only need the id for cross-references.

// ── dnswiz_zone ─────────────────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*zoneDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*zoneDataSource)(nil)
)

func NewZoneDataSource() datasource.DataSource { return &zoneDataSource{} }

type zoneDataSource struct{ client *client.Client }

type zoneDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Active types.Bool   `tfsdk:"active"`
}

func (d *zoneDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (d *zoneDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a zone by apex name.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, MarkdownDescription: "Zone ID."},
			"name": schema.StringAttribute{Required: true, MarkdownDescription: "Apex name of the zone."},
			"active": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the zone is served."},
		},
	}
}

func (d *zoneDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *zoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg zoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zones, err := d.client.ListZones(ctx)
	if err != nil {
		resp.Diagnostics.AddError("list zones", err.Error())
		return
	}
	want := cfg.Name.ValueString()
	for i := range zones {
		if zones[i].Name == want || zones[i].Name == want+"." {
			cfg.ID = types.StringValue(zones[i].ID)
			cfg.Name = types.StringValue(zones[i].Name)
			cfg.Active = types.BoolValue(zones[i].Active)
			resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
			return
		}
	}
	resp.Diagnostics.AddError("zone not found", fmt.Sprintf("no zone with name %q in this tenant", want))
}

// ── dnswiz_pool ─────────────────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*poolDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*poolDataSource)(nil)
)

func NewPoolDataSource() datasource.DataSource { return &poolDataSource{} }

type poolDataSource struct{ client *client.Client }

type poolDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	HealthMonitorID types.String `tfsdk:"health_monitor_id"`
	SelectionMethod types.String `tfsdk:"selection_method"`
}

func (d *poolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (d *poolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a pool by name.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Computed: true},
			"name":              schema.StringAttribute{Required: true},
			"health_monitor_id": schema.StringAttribute{Computed: true},
			"selection_method":  schema.StringAttribute{Computed: true},
		},
	}
}

func (d *poolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *poolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg poolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	items, err := d.client.ListPools(ctx)
	if err != nil {
		resp.Diagnostics.AddError("list pools", err.Error())
		return
	}
	want := cfg.Name.ValueString()
	for i := range items {
		if items[i].Name == want {
			cfg.ID = types.StringValue(items[i].ID)
			cfg.HealthMonitorID = types.StringValue(items[i].HealthMonitorID)
			cfg.SelectionMethod = types.StringValue(items[i].SelectionMethod)
			resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
			return
		}
	}
	resp.Diagnostics.AddError("pool not found", fmt.Sprintf("no pool with name %q in this tenant", want))
}

// ── dnswiz_endpoint ─────────────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*endpointDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*endpointDataSource)(nil)
)

func NewEndpointDataSource() datasource.DataSource { return &endpointDataSource{} }

type endpointDataSource struct{ client *client.Client }

type endpointDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
	Host  types.String `tfsdk:"host"`
	Port  types.Int64  `tfsdk:"port"`
}

func (d *endpointDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (d *endpointDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an endpoint by name.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Computed: true},
			"name":  schema.StringAttribute{Required: true},
			"value": schema.StringAttribute{Computed: true},
			"host":  schema.StringAttribute{Computed: true},
			"port":  schema.Int64Attribute{Computed: true},
		},
	}
}

func (d *endpointDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *endpointDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg endpointDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	items, err := d.client.ListEndpoints(ctx)
	if err != nil {
		resp.Diagnostics.AddError("list endpoints", err.Error())
		return
	}
	want := cfg.Name.ValueString()
	for i := range items {
		if items[i].Name == want {
			cfg.ID = types.StringValue(items[i].ID)
			cfg.Value = types.StringValue(items[i].Value)
			cfg.Host = types.StringValue(items[i].Host)
			cfg.Port = types.Int64Value(int64(items[i].Port))
			resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
			return
		}
	}
	resp.Diagnostics.AddError("endpoint not found", fmt.Sprintf("no endpoint with name %q in this tenant", want))
}

// ── dnswiz_health_monitor ──────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*healthMonitorDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*healthMonitorDataSource)(nil)
)

func NewHealthMonitorDataSource() datasource.DataSource { return &healthMonitorDataSource{} }

type healthMonitorDataSource struct{ client *client.Client }

type healthMonitorDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Kind     types.String `tfsdk:"kind"`
	Path     types.String `tfsdk:"path"`
	IsPreset types.Bool   `tfsdk:"is_preset"`
}

func (d *healthMonitorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_health_monitor"
}

func (d *healthMonitorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a health monitor by name. Use this for the preset monitors dnswiz ships so you can reference them from pools without recreating them.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true},
			"name":      schema.StringAttribute{Required: true},
			"kind":      schema.StringAttribute{Computed: true},
			"path":      schema.StringAttribute{Computed: true},
			"is_preset": schema.BoolAttribute{Computed: true},
		},
	}
}

func (d *healthMonitorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *healthMonitorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg healthMonitorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	items, err := d.client.ListHealthMonitors(ctx)
	if err != nil {
		resp.Diagnostics.AddError("list health monitors", err.Error())
		return
	}
	want := cfg.Name.ValueString()
	for i := range items {
		if items[i].Name == want {
			cfg.ID = types.StringValue(items[i].ID)
			cfg.Kind = types.StringValue(items[i].Kind)
			cfg.Path = types.StringValue(items[i].Path)
			cfg.IsPreset = types.BoolValue(items[i].IsPreset)
			resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
			return
		}
	}
	resp.Diagnostics.AddError("health monitor not found", fmt.Sprintf("no health monitor with name %q in this tenant", want))
}
