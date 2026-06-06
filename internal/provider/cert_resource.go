package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                   = (*certResource)(nil)
	_ resource.ResourceWithConfigure      = (*certResource)(nil)
	_ resource.ResourceWithModifyPlan     = (*certResource)(nil)
)

func NewCertResource() resource.Resource { return &certResource{} }

type certResource struct {
	client *client.Client
}

type certResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Names           types.List   `tfsdk:"names"`
	MinDaysRemaining types.Int64 `tfsdk:"min_days_remaining"`

	Serial        types.String `tfsdk:"serial"`
	SANs          types.List   `tfsdk:"sans"`
	ExpiresAt     types.String `tfsdk:"expires_at"`
	CertPEM       types.String `tfsdk:"cert_pem"`
	IssuerPEM     types.String `tfsdk:"issuer_pem"`
	FullChainPEM  types.String `tfsdk:"fullchain_pem"`
	PrivateKeyPEM types.String `tfsdk:"private_key_pem"`
}

func (r *certResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cert"
}

func (r *certResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Issue a TLS certificate from your dnswiz tenant's configured ACME CA " +
			"(Let's Encrypt by default). dnswiz solves the DNS-01 challenge automatically using the " +
			"zone you already manage with this provider, so no extra wiring is needed.\n\n" +
			"The resource generates an ECDSA P-256 key server-side, signs a CSR with it, runs the " +
			"ACME order, and returns the cert + key in the response. The key is sensitive and stored " +
			"in your Terraform state, so make sure your state backend is encrypted at rest.\n\n" +
			"**Renewal:** set `min_days_remaining` (default 30). When the cert's `expires_at` is " +
			"within that window, the next `terraform plan` will mark the resource for replacement and " +
			"a fresh cert is issued on `apply`.\n\n" +
			"**Idempotency:** every create consumes one ACME issuance from your CA's rate-limit " +
			"window (50/week per registered domain on Let's Encrypt prod). Plan replacements (renewal, " +
			"changing `names`) issue a new cert; they don't reuse the existing one.\n\n" +
			"**Destroy** is a no-op on the server: the cert ledger row stays for audit, the cert " +
			"itself was never persisted server-side anyway. The resource simply drops out of state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cert serial number assigned by the CA. Stable for the life of the cert.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"names": schema.ListAttribute{
				MarkdownDescription: "DNS names to include in the certificate. The first entry becomes " +
					"the primary (CommonName); the rest go in the SAN list. Wildcards like `*.example.com` " +
					"are supported as long as the zone is managed by dnswiz. Changing this list forces " +
					"a new resource (a new cert is issued).",
				Required:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"min_days_remaining": schema.Int64Attribute{
				MarkdownDescription: "When the cert has fewer than this many days left until expiry, the " +
					"next `terraform plan` marks the resource for replacement (renewal). Default `30`. " +
					"Set to `0` to disable automatic renewal. Useful if you'd rather rotate on a cron " +
					"with `terraform apply -replace=...`.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(30),
			},

			"serial": schema.StringAttribute{
				MarkdownDescription: "Cert serial number, hex-encoded as the CA returned it.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sans": schema.ListAttribute{
				MarkdownDescription: "Final SAN list as seen on the issued cert (normalized: lowercased, " +
					"trailing dots stripped). Usually matches `names` exactly.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "Cert expiry timestamp (RFC 3339). The renewal logic in " +
					"`min_days_remaining` compares this to the current time at plan-time.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cert_pem": schema.StringAttribute{
				MarkdownDescription: "PEM-encoded leaf certificate (just the leaf, no chain).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"issuer_pem": schema.StringAttribute{
				MarkdownDescription: "PEM-encoded issuer chain (intermediates only, leaf NOT included).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"fullchain_pem": schema.StringAttribute{
				MarkdownDescription: "PEM-encoded leaf + intermediates, in the order most servers want " +
					"(nginx, Caddy, HAProxy, AWS ELB). This is usually what you wire into `tls_certificate`.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"private_key_pem": schema.StringAttribute{
				MarkdownDescription: "PEM-encoded ECDSA P-256 private key. Stored as a sensitive value " +
					"in Terraform state; use an encrypted state backend.",
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *certResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *certResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan certResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var names []string
	resp.Diagnostics.Append(plan.Names.ElementsAs(ctx, &names, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(names) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("names"),
			"empty names list",
			"At least one DNS name is required.",
		)
		return
	}

	iss, err := r.client.IssueCert(ctx, client.IssueRequest{Names: names})
	if err != nil {
		resp.Diagnostics.AddError("issue cert", err.Error())
		return
	}

	plan.ID = types.StringValue(iss.Serial)
	plan.Serial = types.StringValue(iss.Serial)
	plan.ExpiresAt = types.StringValue(iss.ExpiresAt.UTC().Format(time.RFC3339))
	plan.CertPEM = types.StringValue(iss.CertPEM)
	plan.IssuerPEM = types.StringValue(iss.IssuerPEM)
	plan.FullChainPEM = types.StringValue(iss.FullChainPEM)
	plan.PrivateKeyPEM = types.StringValue(iss.KeyPEM)
	sansList, d := types.ListValueFrom(ctx, types.StringType, iss.SANs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.SANs = sansList

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read is intentionally a no-op past the initial issuance. The cert
// material lives in state; the server-side ledger row is audit-only
// and doesn't expose the leaf/key by id. There is no useful drift to
// detect: a cert in state is either still valid (we keep it) or
// approaching expiry (ModifyPlan handles that).
func (r *certResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update only runs for in-place changes. The only mutable input
// (`min_days_remaining`) is purely client-side; no API call needed.
// Any change to `names` triggers RequiresReplace, so this path never
// sees that case.
func (r *certResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan certResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op on the server. The ledger row stays for audit;
// the leaf + key were never persisted server-side. The resource just
// drops out of Terraform state.
func (r *certResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ModifyPlan triggers a replacement when the cert is within
// `min_days_remaining` of expiry. The hashicorp/tls provider uses the
// same pattern (`early_renewal_hours`). Setting `min_days_remaining`
// to 0 disables this and renewal becomes a manual `-replace=`.
func (r *certResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() {
		return // creating, nothing to renew
	}
	var state certResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.ExpiresAt.IsNull() || state.ExpiresAt.IsUnknown() {
		return
	}
	expires, err := time.Parse(time.RFC3339, state.ExpiresAt.ValueString())
	if err != nil {
		return // unparseable expiry: leave as-is, don't churn
	}
	minDays := int64(30)
	if !state.MinDaysRemaining.IsNull() && !state.MinDaysRemaining.IsUnknown() {
		minDays = state.MinDaysRemaining.ValueInt64()
	}
	if minDays <= 0 {
		return
	}
	remaining := time.Until(expires).Hours() / 24
	if remaining > float64(minDays) {
		return
	}
	resp.RequiresReplace = []path.Path{path.Root("names")}
	resp.Diagnostics.AddAttributeWarning(
		path.Root("names"),
		"cert nearing expiry; will be renewed",
		fmt.Sprintf("Certificate expires at %s (%0.1f days remaining, below the configured min_days_remaining=%d). A new cert will be issued on apply.",
			expires.UTC().Format(time.RFC3339), remaining, minDays),
	)
}
