package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ datasource.DataSource              = (*ipamAvailableIPDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*ipamAvailableIPDataSource)(nil)
)

func NewIPAMAvailableIPDataSource() datasource.DataSource {
	return &ipamAvailableIPDataSource{}
}

type ipamAvailableIPDataSource struct{ client *client.Client }

type ipamAvailableIPModel struct {
	NetworkID types.String `tfsdk:"network_id"`
	Address   types.String `tfsdk:"address"`
}

func (d *ipamAvailableIPDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipam_available_ip"
}

func (d *ipamAvailableIPDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds the next free host address in a network, without allocating it. It's a read-time peek — for concurrency-safe allocation, create a `dnswiz_ipam_ip_address` with no `address` instead.",
		Attributes: map[string]schema.Attribute{
			"network_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the network to look in.",
			},
			"address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The next free host address, e.g. `10.1.5.10`.",
			},
		},
	}
}

func (d *ipamAvailableIPDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ipamAvailableIPDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg ipamAvailableIPModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addr, err := d.client.NextAvailableIP(ctx, cfg.NetworkID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("find available ip", err.Error())
		return
	}
	cfg.Address = types.StringValue(addr)
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
