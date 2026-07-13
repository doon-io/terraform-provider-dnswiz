// Package provider is the canonical implementation of the dnswiz Terraform
// provider. Resources / data sources live in sibling files in this package.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

// dnswizProvider implements provider.Provider. The version is injected at
// build time and surfaces in user-agent strings so the dnswiz server can
// see which version is talking to it.
type dnswizProvider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &dnswizProvider{version: version}
	}
}

func (p *dnswizProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "dnswiz"
	resp.Version = p.version
}

// dnswizProviderModel mirrors the provider {} block on the user's side.
// Nullables let us distinguish "user explicitly set empty" from "user
// did not set"; we fall back to env vars only in the latter case.
type dnswizProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func (p *dnswizProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage [dnswiz](https://dnswiz.app) end-to-end as code: " +
			"authoritative DNS zones and records, GSLB pools and members, health monitors, " +
			"zone security policies, notification channels, and TLS certificates issued via " +
			"ACME with DNS-01 solved automatically.\n\n" +
			"dnswiz is a hosted GSLB + authoritative DNS service. Once a zone is delegated to " +
			"`ns1.dnswiz.app` and `ns2.dnswiz.app`, this provider becomes the single source of " +
			"truth for everything about it: A/AAAA, GSLB routing (failover, weighted, geo, " +
			"latency, canary), uptime monitors, the firewall that decides which queries to " +
			"answer, and the TLS certs your apps actually serve.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL of the dnswiz API, including the `/api` path prefix. Defaults to `https://console.dnswiz.app/api`. Override for self-hosted installs. Can be set via the `DNSWIZ_ENDPOINT` env var.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "API key for the dnswiz account. Generate one at Settings → API keys. Can be set via the `DNSWIZ_API_KEY` env var.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *dnswizProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg dnswizProviderModel
	if diag := req.Config.Get(ctx, &cfg); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	// Resolve endpoint + api_key with env-var fallback. Tested order
	// matches Terraform conventions: explicit config wins over env.
	// The /api suffix is required because the production reverse proxy
	// routes /api/* to the API backend and /v1/* directly to the
	// browser SPA.
	endpoint := stringOrEnv(cfg.Endpoint, "DNSWIZ_ENDPOINT", "https://console.dnswiz.app/api")
	apiKey := stringOrEnv(cfg.APIKey, "DNSWIZ_API_KEY", "")

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing dnswiz API key",
			"The dnswiz provider requires an api_key. Set it in the provider block or via the DNSWIZ_API_KEY environment variable.",
		)
		return
	}

	c := client.New(endpoint, apiKey, "terraform-provider-dnswiz/"+p.version)

	// Same client instance for resources and data sources so they
	// share the underlying HTTP connection pool.
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *dnswizProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewZoneResource,
		NewRecordResource,
		NewPoolResource,
		NewPoolMemberResource,
		NewEndpointResource,
		NewHealthMonitorResource,
		NewNotificationChannelResource,
		NewZonePolicyResource,
		NewCertResource,
		NewIPAMBlockResource,
		NewIPAMNetworkResource,
		NewIPAMIPAddressResource,
	}
}

func (p *dnswizProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewZoneDataSource,
		NewPoolDataSource,
		NewEndpointDataSource,
		NewHealthMonitorDataSource,
		NewIPAMAvailableSubnetDataSource,
		NewIPAMAvailableIPDataSource,
	}
}

// stringOrEnv returns the value in `v` if set, otherwise the env var, otherwise
// the default.
func stringOrEnv(v types.String, envKey, def string) string {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		return v.ValueString()
	}
	if env := os.Getenv(envKey); env != "" {
		return env
	}
	return def
}
