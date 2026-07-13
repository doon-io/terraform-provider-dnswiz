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
	_ datasource.DataSource              = (*ipamAvailableSubnetDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*ipamAvailableSubnetDataSource)(nil)
)

func NewIPAMAvailableSubnetDataSource() datasource.DataSource {
	return &ipamAvailableSubnetDataSource{}
}

type ipamAvailableSubnetDataSource struct{ client *client.Client }

type ipamAvailableSubnetModel struct {
	BlockID      types.String `tfsdk:"block_id"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	CIDR         types.String `tfsdk:"cidr"`
}

func (d *ipamAvailableSubnetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipam_available_subnet"
}

func (d *ipamAvailableSubnetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds the next free aligned subnet of a given prefix length inside a block, without allocating it. Use it to feed a `dnswiz_ipam_block` or network CIDR. Note: this is a read-time peek — two parallel applies could pick the same range; for concurrency-safe allocation prefer creating the child with an explicit CIDR you manage.",
		Attributes: map[string]schema.Attribute{
			"block_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the block to carve from.",
			},
			"prefix_length": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Desired subnet prefix length (must be longer than the block's), for example `24`.",
			},
			"cidr": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The next free aligned subnet of that size, for example `10.0.5.0/24`.",
			},
		},
	}
}

func (d *ipamAvailableSubnetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ipamAvailableSubnetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg ipamAvailableSubnetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cidr, err := d.client.NextAvailableSubnet(ctx, cfg.BlockID.ValueString(), int(cfg.PrefixLength.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("find available subnet", err.Error())
		return
	}
	cfg.CIDR = types.StringValue(cidr)
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
