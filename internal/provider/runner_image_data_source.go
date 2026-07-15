package provider

import (
	"context"
	"fmt"

	"github.com/WarpBuilds/terraform-provider-warpbuild/internal/wbclient"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &runnerImageDataSource{}
	_ datasource.DataSourceWithConfigure = &runnerImageDataSource{}
)

type runnerImageDataSource struct {
	client *wbclient.APIClient
}

type runnerImageDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Alias   types.String `tfsdk:"alias"`
	StackID types.String `tfsdk:"stack_id"`
	OS      types.String `tfsdk:"os"`
	Arch    types.String `tfsdk:"arch"`
	Type    types.String `tfsdk:"type"`
	Status  types.String `tfsdk:"status"`
}

func NewRunnerImageDataSource() datasource.DataSource {
	return &runnerImageDataSource{}
}

func (d *runnerImageDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_image"
}

func (d *runnerImageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing runner image by alias.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Runner image ID.",
				Computed:            true,
			},
			"alias": schema.StringAttribute{
				MarkdownDescription: "Image alias to look up.",
				Required:            true,
			},
			"stack_id": schema.StringAttribute{
				MarkdownDescription: "Stack the image belongs to.",
				Computed:            true,
			},
			"os": schema.StringAttribute{
				MarkdownDescription: "Operating system.",
				Computed:            true,
			},
			"arch": schema.StringAttribute{
				MarkdownDescription: "CPU architecture.",
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Image type (e.g. `byoc_ami`).",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Image status.",
				Computed:            true,
			},
		},
	}
}

func (d *runnerImageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*wbclient.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *wbclient.APIClient, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *runnerImageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config runnerImageDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, httpResp, err := d.client.V1RunnerImagesAPI.ListRunnerImages(ctx).
		Alias(config.Alias.ValueString()).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to list runner images", apiError(httpResp, err))
		return
	}

	if out == nil || len(out.RunnerImages) == 0 {
		resp.Diagnostics.AddError("Runner image not found",
			fmt.Sprintf("no runner image with alias %q", config.Alias.ValueString()))
		return
	}
	if len(out.RunnerImages) > 1 {
		resp.Diagnostics.AddError("Multiple runner images match",
			fmt.Sprintf("%d runner images match alias %q", len(out.RunnerImages), config.Alias.ValueString()))
		return
	}

	image := out.RunnerImages[0]
	config.ID = types.StringValue(image.Id)
	config.Alias = types.StringPointerValue(image.Alias)
	config.StackID = types.StringPointerValue(image.StackId)
	config.OS = types.StringPointerValue(image.Os)
	config.Arch = types.StringPointerValue(image.Arch)
	config.Type = types.StringPointerValue(image.Type)
	config.Status = types.StringPointerValue(image.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
