package provider

import (
	"context"
	"os"

	"github.com/WarpBuilds/terraform-provider-warpbuild/internal/wbclient"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultAPIEndpoint = "https://api.warpbuild.com/api/v1"

var _ provider.Provider = &warpbuildProvider{}

type warpbuildProvider struct {
	version string
}

type warpbuildProviderModel struct {
	APIKey      types.String `tfsdk:"api_key"`
	APIEndpoint types.String `tfsdk:"api_endpoint"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &warpbuildProvider{version: version}
	}
}

func (p *warpbuildProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "warpbuild"
	resp.Version = p.version
}

func (p *warpbuildProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage WarpBuild runner infrastructure via the " +
			"[automation API](https://www.warpbuild.com/docs/ci/api-keys/automation).",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "WarpBuild API key with the `ci` scope. May also be set via the " +
					"`WARPBUILD_API_KEY` environment variable.",
				Optional:  true,
				Sensitive: true,
			},
			"api_endpoint": schema.StringAttribute{
				MarkdownDescription: "WarpBuild API endpoint. Defaults to `" + defaultAPIEndpoint + "`. " +
					"May also be set via the `WARPBUILD_API_ENDPOINT` environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *warpbuildProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config warpbuildProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("WARPBUILD_API_KEY")
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing WarpBuild API key",
			"Set the api_key provider attribute or the WARPBUILD_API_KEY environment variable. "+
				"API keys are created at https://app.warpbuild.com/settings/api-keys.",
		)
		return
	}

	endpoint := os.Getenv("WARPBUILD_API_ENDPOINT")
	if !config.APIEndpoint.IsNull() {
		endpoint = config.APIEndpoint.ValueString()
	}
	if endpoint == "" {
		endpoint = defaultAPIEndpoint
	}

	cfg := wbclient.NewConfiguration()
	cfg.Servers = wbclient.ServerConfigurations{{URL: endpoint}}
	cfg.UserAgent = "terraform-provider-warpbuild/" + p.version
	cfg.AddDefaultHeader("Authorization", "Bearer "+apiKey)

	client := wbclient.NewAPIClient(cfg)
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *warpbuildProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRunnerImageResource,
		NewRunnerResource,
	}
}

func (p *warpbuildProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewStackDataSource,
		NewRunnerImageDataSource,
	}
}
