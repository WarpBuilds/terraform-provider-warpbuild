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
	_ datasource.DataSource              = &stackDataSource{}
	_ datasource.DataSourceWithConfigure = &stackDataSource{}
)

type stackDataSource struct {
	client *wbclient.APIClient
}

type stackDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	Alias  types.String `tfsdk:"alias"`
	Kind   types.String `tfsdk:"kind"`
	Region types.String `tfsdk:"region"`
	Status types.String `tfsdk:"status"`
}

func NewStackDataSource() datasource.DataSource {
	return &stackDataSource{}
}

func (d *stackDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack"
}

func (d *stackDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a single stack (runner infrastructure deployment). " +
			"Use its `id` as `stack_id` on runner images and `provider_id` on runners.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stack ID.",
				Computed:            true,
			},
			"alias": schema.StringAttribute{
				MarkdownDescription: "Stack alias to look up.",
				Optional:            true,
				Computed:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Stack kind filter. Defaults to `ec2`.",
				Optional:            true,
				Computed:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Region filter (e.g. `us-east-1`).",
				Optional:            true,
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Stack status.",
				Computed:            true,
			},
		},
	}
}

func (d *stackDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *stackDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config stackDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	kind := "ec2"
	if !config.Kind.IsNull() {
		kind = config.Kind.ValueString()
	}

	listReq := d.client.V1StacksAPI.ListStacks(ctx).Kind(kind)
	if !config.Alias.IsNull() {
		listReq = listReq.Alias(config.Alias.ValueString())
	}
	if !config.Region.IsNull() {
		listReq = listReq.Region(config.Region.ValueString())
	}

	stacks, httpResp, err := listReq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to list stacks", apiError(httpResp, err))
		return
	}

	if len(stacks) == 0 {
		resp.Diagnostics.AddError("Stack not found", "no stack matches the given filters")
		return
	}
	if len(stacks) > 1 {
		resp.Diagnostics.AddError("Multiple stacks match",
			fmt.Sprintf("%d stacks match the given filters; add alias or region to disambiguate", len(stacks)))
		return
	}

	stack := stacks[0]
	config.ID = types.StringPointerValue(stack.Id)
	config.Alias = types.StringPointerValue(stack.Alias)
	config.Kind = types.StringValue(stack.Kind)
	config.Region = types.StringPointerValue(stack.Region)
	config.Status = types.StringPointerValue(stack.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
