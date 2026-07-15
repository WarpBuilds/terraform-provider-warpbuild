package provider

import (
	"context"
	"fmt"

	"github.com/WarpBuilds/terraform-provider-warpbuild/internal/wbclient"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// runnerImageTypeByocAMI is the only image type this provider manages. Other
// image flows (container, warpbuild_managed, snapshot) are intentionally not
// expressible here.
const runnerImageTypeByocAMI = "byoc_ami"

var (
	_ resource.Resource                = &runnerImageResource{}
	_ resource.ResourceWithConfigure   = &runnerImageResource{}
	_ resource.ResourceWithImportState = &runnerImageResource{}
)

type runnerImageResource struct {
	client *wbclient.APIClient
}

type runnerImageResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	Alias                    types.String `tfsdk:"alias"`
	StackID                  types.String `tfsdk:"stack_id"`
	AmiID                    types.String `tfsdk:"ami_id"`
	PurgeImageVersionsOffset types.Int64  `tfsdk:"purge_image_versions_offset"`
	OS                       types.String `tfsdk:"os"`
	Arch                     types.String `tfsdk:"arch"`
	Status                   types.String `tfsdk:"status"`
}

func NewRunnerImageResource() resource.Resource {
	return &runnerImageResource{}
}

func (r *runnerImageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_image"
}

func (r *runnerImageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A BYOC (bring-your-own-cloud) AWS AMI runner image. " +
			"The image's OS, architecture and root device are derived from the AMI by WarpBuild. " +
			"Updating `ami_id` creates a new image version in place; older versions are purged " +
			"automatically.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Runner image ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the image. Unique within the organization and immutable.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"stack_id": schema.StringAttribute{
				MarkdownDescription: "ID of the EC2 stack the image belongs to. " +
					"Use the `warpbuild_stack` data source to look this up.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ami_id": schema.StringAttribute{
				MarkdownDescription: "AWS AMI ID of the customer-owned image. " +
					"Changing this creates a new image version in place.",
				Required: true,
			},
			"purge_image_versions_offset": schema.Int64Attribute{
				MarkdownDescription: "Number of image versions kept before older ones are purged. Managed by WarpBuild.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"os": schema.StringAttribute{
				MarkdownDescription: "Operating system, derived from the AMI.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"arch": schema.StringAttribute{
				MarkdownDescription: "CPU architecture, derived from the AMI.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Image status.",
				Computed:            true,
			},
		},
	}
}

func (r *runnerImageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*wbclient.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *wbclient.APIClient, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *runnerImageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runnerImageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := wbclient.CommonsCreateRunnerImageInput{
		Alias:   plan.Alias.ValueString(),
		Type:    runnerImageTypeByocAMI,
		StackId: plan.StackID.ValueStringPointer(),
		ByocAmi: &wbclient.CommonsByocAMI{
			AmiId: plan.AmiID.ValueString(),
		},
	}

	image, httpResp, err := r.client.V1RunnerImagesAPI.CreateRunnerImage(ctx).Body(input).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to create runner image", apiError(httpResp, err))
		return
	}

	r.setState(&plan, image)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *runnerImageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerImageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	image, httpResp, err := r.client.V1RunnerImagesAPI.GetRunnerImage(ctx, state.ID.ValueString()).Execute()
	if err != nil {
		if isNotFound(httpResp, err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read runner image", apiError(httpResp, err))
		return
	}

	r.setState(&state, image)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerImageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runnerImageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := wbclient.CommonsUpdateRunnerImageInput{
		ByocAmi: &wbclient.CommonsByocAMI{
			AmiId: plan.AmiID.ValueString(),
		},
	}

	image, httpResp, err := r.client.V1RunnerImagesAPI.UpdateRunnerImage(ctx, plan.ID.ValueString()).Body(input).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to update runner image", apiError(httpResp, err))
		return
	}

	r.setState(&plan, image)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *runnerImageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runnerImageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, httpResp, err := r.client.V1RunnerImagesAPI.DeleteRunnerImage(ctx, state.ID.ValueString()).Execute()
	if err != nil {
		if isNotFound(httpResp, err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete runner image", apiError(httpResp, err))
	}
}

func (r *runnerImageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *runnerImageResource) setState(m *runnerImageResourceModel, image *wbclient.CommonsRunnerImage) {
	m.ID = types.StringValue(image.Id)
	m.Alias = types.StringPointerValue(image.Alias)
	m.StackID = types.StringPointerValue(image.StackId)
	m.OS = types.StringPointerValue(image.Os)
	m.Arch = types.StringPointerValue(image.Arch)
	m.Status = types.StringPointerValue(image.Status)
	if image.ByocAmi != nil {
		m.AmiID = types.StringValue(image.ByocAmi.AmiId)
	}
	if image.Settings != nil && image.Settings.PurgeImageVersionsOffset != nil {
		m.PurgeImageVersionsOffset = types.Int64Value(int64(*image.Settings.PurgeImageVersionsOffset))
	} else {
		m.PurgeImageVersionsOffset = types.Int64Null()
	}
}
