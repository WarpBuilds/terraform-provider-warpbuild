package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/WarpBuilds/terraform-provider-warpbuild/internal/wbclient"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.Resource                = &runnerResource{}
	_ resource.ResourceWithConfigure   = &runnerResource{}
	_ resource.ResourceWithImportState = &runnerResource{}
)

type runnerResource struct {
	client *wbclient.APIClient
}

type runnerResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	ProviderID    types.String `tfsdk:"provider_id"`
	PoolSize      types.Int64  `tfsdk:"pool_size"`
	Labels        types.Set    `tfsdk:"labels"`
	Configuration types.Object `tfsdk:"configuration"`
}

type runnerConfigurationModel struct {
	CapacityType types.String `tfsdk:"capacity_type"`
	Image        types.String `tfsdk:"image"`
	Storage      types.Object `tfsdk:"storage"`
	ByocSku      types.Object `tfsdk:"byoc_sku"`
}

type runnerStorageModel struct {
	Tier       types.String `tfsdk:"tier"`
	Size       types.Int64  `tfsdk:"size"`
	Iops       types.Int64  `tfsdk:"iops"`
	Throughput types.Int64  `tfsdk:"throughput"`
}

type runnerByocSkuModel struct {
	Arch           types.String `tfsdk:"arch"`
	InstanceTypes  types.List   `tfsdk:"instance_types"`
	RoleArn        types.String `tfsdk:"role_arn"`
	IsPublic       types.Bool   `tfsdk:"is_public"`
	Imdsv2Required types.Bool   `tfsdk:"imdsv2_required"`
}

var runnerStorageAttrTypes = map[string]attr.Type{
	"tier":       types.StringType,
	"size":       types.Int64Type,
	"iops":       types.Int64Type,
	"throughput": types.Int64Type,
}

var runnerByocSkuAttrTypes = map[string]attr.Type{
	"arch":            types.StringType,
	"instance_types":  types.ListType{ElemType: types.StringType},
	"role_arn":        types.StringType,
	"is_public":       types.BoolType,
	"imdsv2_required": types.BoolType,
}

var runnerConfigurationAttrTypes = map[string]attr.Type{
	"capacity_type": types.StringType,
	"image":         types.StringType,
	"storage":       types.ObjectType{AttrTypes: runnerStorageAttrTypes},
	"byoc_sku":      types.ObjectType{AttrTypes: runnerByocSkuAttrTypes},
}

func NewRunnerResource() resource.Resource {
	return &runnerResource{}
}

func (r *runnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (r *runnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A custom BYOC runner set. Jobs request it by label; WarpBuild provisions " +
			"EC2 instances into the referenced stack using the configured image and instance types.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Runner ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the runner, unique within the organization. " +
					"Lowercase alphanumerics and hyphens.",
				Required: true,
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "ID of the stack the runner provisions instances into. " +
					"Use the `warpbuild_stack` data source to look this up.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pool_size": schema.Int64Attribute{
				MarkdownDescription: "Number of warm pool instances.",
				Required:            true,
			},
			"labels": schema.SetAttribute{
				MarkdownDescription: "Labels used to select this runner in CI workflows. Defaults to the runner name.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Hardware and image configuration for the runner.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"capacity_type": schema.StringAttribute{
						MarkdownDescription: "`ondemand` or `spot`.",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("ondemand"),
						Validators: []validator.String{
							stringvalidator.OneOf("ondemand", "spot"),
						},
					},
					"image": schema.StringAttribute{
						MarkdownDescription: "Runner image ID (e.g. from a `warpbuild_runner_image` resource) " +
							"or a WarpBuild stock image alias like `ubuntu-2404`.",
						Required: true,
					},
					"storage": schema.SingleNestedAttribute{
						MarkdownDescription: "Instance storage configuration. Server defaults apply when omitted.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.UseStateForUnknown(),
						},
						Attributes: map[string]schema.Attribute{
							"tier": schema.StringAttribute{
								MarkdownDescription: "Storage tier: `low`, `medium`, `high`, `extreme` or `custom`.",
								Optional:            true,
								Computed:            true,
								Validators: []validator.String{
									stringvalidator.OneOf("low", "medium", "high", "extreme", "custom"),
								},
							},
							"size": schema.Int64Attribute{
								MarkdownDescription: "Disk size in GB.",
								Optional:            true,
								Computed:            true,
							},
							"iops": schema.Int64Attribute{
								MarkdownDescription: "Provisioned IOPS.",
								Optional:            true,
								Computed:            true,
							},
							"throughput": schema.Int64Attribute{
								MarkdownDescription: "Disk throughput in MB/s.",
								Optional:            true,
								Computed:            true,
							},
						},
					},
					"byoc_sku": schema.SingleNestedAttribute{
						MarkdownDescription: "EC2 instance selection for the BYOC runner.",
						Required:            true,
						Attributes: map[string]schema.Attribute{
							"arch": schema.StringAttribute{
								MarkdownDescription: "CPU architecture: `x64` or `arm64`.",
								Required:            true,
								Validators: []validator.String{
									stringvalidator.OneOf("x64", "arm64"),
								},
							},
							"instance_types": schema.ListAttribute{
								MarkdownDescription: "Acceptable EC2 instance types, in order of preference.",
								ElementType:         types.StringType,
								Required:            true,
							},
							"role_arn": schema.StringAttribute{
								MarkdownDescription: "AWS IAM role ARN attached to runner instances.",
								Optional:            true,
							},
							"is_public": schema.BoolAttribute{
								MarkdownDescription: "Whether instances get public IPs.",
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
							},
							"imdsv2_required": schema.BoolAttribute{
								MarkdownDescription: "Enforce IMDSv2 (disable IMDSv1) on instances.",
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
							},
						},
					},
				},
			},
		},
	}
}

func (r *runnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *runnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runnerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configuration, diags := expandRunnerConfiguration(ctx, plan.Configuration)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := wbclient.CommonsSetupRunnerInput{
		Name:          plan.Name.ValueStringPointer(),
		ProviderId:    plan.ProviderID.ValueStringPointer(),
		PoolSize:      int32Ptr(plan.PoolSize.ValueInt64()),
		Configuration: configuration,
	}
	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		resp.Diagnostics.Append(plan.Labels.ElementsAs(ctx, &input.Labels, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	runner, httpResp, err := r.client.V1RunnersAPI.SetupRunner(ctx).Body(input).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to create runner", apiError(httpResp, err))
		return
	}

	resp.Diagnostics.Append(r.setState(ctx, &plan, runner)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *runnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	runner, httpResp, err := r.client.V1RunnersAPI.GetRunner(ctx, state.ID.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read runner", apiError(httpResp, err))
		return
	}

	resp.Diagnostics.Append(r.setState(ctx, &state, runner)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runnerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configuration, diags := expandRunnerConfiguration(ctx, plan.Configuration)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := wbclient.CommonsUpdateRunnerInput{
		Name:          plan.Name.ValueStringPointer(),
		PoolSize:      int32Ptr(plan.PoolSize.ValueInt64()),
		Configuration: configuration,
	}
	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		resp.Diagnostics.Append(plan.Labels.ElementsAs(ctx, &input.Labels, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	runner, httpResp, err := r.client.V1RunnersAPI.UpdateRunner(ctx, plan.ID.ValueString()).Body(input).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to update runner", apiError(httpResp, err))
		return
	}

	resp.Diagnostics.Append(r.setState(ctx, &plan, runner)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *runnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, httpResp, err := r.client.V1RunnersAPI.DeleteRunner(ctx, state.ID.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Failed to delete runner", apiError(httpResp, err))
	}
}

func (r *runnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func expandRunnerConfiguration(ctx context.Context, obj types.Object) (*wbclient.CommonsRunnerSetConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics
	var cfg runnerConfigurationModel
	diags.Append(obj.As(ctx, &cfg, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	out := &wbclient.CommonsRunnerSetConfiguration{
		CapacityType: cfg.CapacityType.ValueStringPointer(),
		Image:        cfg.Image.ValueStringPointer(),
	}

	if !cfg.Storage.IsNull() && !cfg.Storage.IsUnknown() {
		var storage runnerStorageModel
		diags.Append(cfg.Storage.As(ctx, &storage, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		out.Storage = &wbclient.CommonsStorage{
			Tier: storage.Tier.ValueStringPointer(),
		}
		if !storage.Size.IsNull() && !storage.Size.IsUnknown() {
			out.Storage.Size = int32Ptr(storage.Size.ValueInt64())
		}
		if !storage.Iops.IsNull() && !storage.Iops.IsUnknown() {
			out.Storage.Iops = int32Ptr(storage.Iops.ValueInt64())
		}
		if !storage.Throughput.IsNull() && !storage.Throughput.IsUnknown() {
			out.Storage.Throughput = int32Ptr(storage.Throughput.ValueInt64())
		}
	}

	if !cfg.ByocSku.IsNull() && !cfg.ByocSku.IsUnknown() {
		var sku runnerByocSkuModel
		diags.Append(cfg.ByocSku.As(ctx, &sku, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		out.ByocSku = &wbclient.CommonsByocSku{
			Arch:           sku.Arch.ValueStringPointer(),
			RoleArn:        sku.RoleArn.ValueStringPointer(),
			IsPublic:       sku.IsPublic.ValueBoolPointer(),
			Imdsv2Required: sku.Imdsv2Required.ValueBoolPointer(),
		}
		diags.Append(sku.InstanceTypes.ElementsAs(ctx, &out.ByocSku.InstanceTypes, false)...)
		if diags.HasError() {
			return nil, diags
		}
	}

	return out, diags
}

func (r *runnerResource) setState(ctx context.Context, m *runnerResourceModel, runner *wbclient.CommonsRunner) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringPointerValue(runner.Id)
	m.Name = types.StringPointerValue(runner.Name)
	m.ProviderID = types.StringPointerValue(runner.ProviderId)

	labels, d := types.SetValueFrom(ctx, types.StringType, runner.Labels)
	diags.Append(d...)
	m.Labels = labels

	if runner.Configuration == nil {
		m.Configuration = types.ObjectNull(runnerConfigurationAttrTypes)
		return diags
	}
	apiCfg := runner.Configuration

	storage := types.ObjectNull(runnerStorageAttrTypes)
	if apiCfg.Storage != nil {
		s, d := types.ObjectValueFrom(ctx, runnerStorageAttrTypes, runnerStorageModel{
			Tier:       types.StringPointerValue(apiCfg.Storage.Tier),
			Size:       int64PtrValue(apiCfg.Storage.Size),
			Iops:       int64PtrValue(apiCfg.Storage.Iops),
			Throughput: int64PtrValue(apiCfg.Storage.Throughput),
		})
		diags.Append(d...)
		storage = s
	}

	byocSku := types.ObjectNull(runnerByocSkuAttrTypes)
	if apiCfg.ByocSku != nil {
		instanceTypes, d := types.ListValueFrom(ctx, types.StringType, apiCfg.ByocSku.InstanceTypes)
		diags.Append(d...)
		s, d := types.ObjectValueFrom(ctx, runnerByocSkuAttrTypes, runnerByocSkuModel{
			Arch:           types.StringPointerValue(apiCfg.ByocSku.Arch),
			InstanceTypes:  instanceTypes,
			RoleArn:        types.StringPointerValue(apiCfg.ByocSku.RoleArn),
			IsPublic:       types.BoolPointerValue(apiCfg.ByocSku.IsPublic),
			Imdsv2Required: types.BoolPointerValue(apiCfg.ByocSku.Imdsv2Required),
		})
		diags.Append(d...)
		byocSku = s
	}

	configuration, d := types.ObjectValueFrom(ctx, runnerConfigurationAttrTypes, runnerConfigurationModel{
		CapacityType: types.StringPointerValue(apiCfg.CapacityType),
		Image:        types.StringPointerValue(apiCfg.Image),
		Storage:      storage,
		ByocSku:      byocSku,
	})
	diags.Append(d...)
	m.Configuration = configuration

	return diags
}
