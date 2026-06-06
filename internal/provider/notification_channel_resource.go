package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/doon-io/terraform-provider-dnswiz/internal/client"
)

var (
	_ resource.Resource                = (*notificationChannelResource)(nil)
	_ resource.ResourceWithConfigure   = (*notificationChannelResource)(nil)
	_ resource.ResourceWithImportState = (*notificationChannelResource)(nil)
)

func NewNotificationChannelResource() resource.Resource {
	return &notificationChannelResource{}
}

type notificationChannelResource struct {
	client *client.Client
}

type notificationChannelResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Kind   types.String `tfsdk:"kind"`
	Target types.String `tfsdk:"target"`
	Secret types.String `tfsdk:"secret"`
	Events types.List   `tfsdk:"events"`
	Active types.Bool   `tfsdk:"active"`
}

func (r *notificationChannelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_channel"
}

func (r *notificationChannelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A destination for dnswiz notifications. Today only webhook is supported.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable channel name.",
				Required:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Channel kind. Only `webhook` is supported.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("webhook"),
				},
			},
			"target": schema.StringAttribute{
				MarkdownDescription: "Webhook URL.",
				Required:            true,
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "HMAC signing secret returned by dnswiz on create. The server never returns it again, so this attribute stays null on subsequent reads.",
				Computed:            true,
				Sensitive:           true,
			},
			"events": schema.ListAttribute{
				MarkdownDescription: "Event types that fire this channel.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the channel is dispatched. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *notificationChannelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *notificationChannelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan notificationChannelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var events []string
	resp.Diagnostics.Append(plan.Events.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.CreateNotificationChannel(ctx, client.NotificationChannel{
		Name:   plan.Name.ValueString(),
		Kind:   plan.Kind.ValueString(),
		Target: plan.Target.ValueString(),
		Events: events,
		Active: plan.Active.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("create notification channel", err.Error())
		return
	}
	state, diags := fromAPINotificationChannel(ctx, got)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *notificationChannelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prev notificationChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prev)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetNotificationChannel(ctx, prev.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read notification channel", err.Error())
		return
	}
	state, diags := fromAPINotificationChannel(ctx, got)
	state.Secret = prev.Secret // server omits secret on read; keep the value we got at create
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *notificationChannelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prev notificationChannelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prev)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var events []string
	resp.Diagnostics.Append(plan.Events.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	target := plan.Target.ValueString()
	active := plan.Active.ValueBool()
	got, err := r.client.UpdateNotificationChannel(ctx, plan.ID.ValueString(), client.NotificationChannelUpdate{
		Name:   &name,
		Target: &target,
		Events: events,
		Active: &active,
	})
	if err != nil {
		resp.Diagnostics.AddError("update notification channel", err.Error())
		return
	}
	state, diags := fromAPINotificationChannel(ctx, got)
	state.Secret = prev.Secret
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *notificationChannelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state notificationChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotificationChannel(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("delete notification channel", err.Error())
		return
	}
}

func (r *notificationChannelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func fromAPINotificationChannel(ctx context.Context, c *client.NotificationChannel) (notificationChannelResourceModel, diag.Diagnostics) {
	events, diags := types.ListValueFrom(ctx, types.StringType, c.Events)
	m := notificationChannelResourceModel{
		ID:     types.StringValue(c.ID),
		Name:   types.StringValue(c.Name),
		Kind:   types.StringValue(c.Kind),
		Target: types.StringValue(c.Target),
		Active: types.BoolValue(c.Active),
		Events: events,
	}
	if c.Secret != "" {
		m.Secret = types.StringValue(c.Secret)
	} else {
		m.Secret = types.StringNull()
	}
	return m, diags
}
