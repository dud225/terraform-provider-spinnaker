package provider

import (
  "context"
  "io"
  "reflect"

  "github.com/hashicorp/terraform-plugin-framework/datasource"
  "github.com/hashicorp/terraform-plugin-framework/diag"
  "github.com/hashicorp/terraform-plugin-framework/path"
  "github.com/hashicorp/terraform-plugin-framework/provider"
  "github.com/hashicorp/terraform-plugin-framework/resource"
  "github.com/hashicorp/terraform-plugin-framework/tfsdk"
  "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

  "github.com/spinnaker/spin/cmd/gateclient"
  "github.com/spinnaker/spin/cmd/output"
)

var _ provider.ProviderWithMetadata = (*spinnakerProvider)(nil)

type spinnakerProvider struct {
  version string
}

type providerData struct {
  SpinConfig       types.String `tfsdk:"spin_config"`
  IgnoreCertErrors types.Bool   `tfsdk:"ignore_cert_errors"`
  DefaultHeaders   types.String `tfsdk:"default_headers"`
}

func New(version string) func() provider.Provider {
  return func() provider.Provider {
    return &spinnakerProvider{
      version: version,
    }
  }
}

func (p *spinnakerProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
  resp.TypeName = "spinnaker"
  resp.Version = p.version
}

func (p *spinnakerProvider) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
  return tfsdk.Schema{
    Description:         "Spinnaker provider configuration",
    MarkdownDescription: "Spinnaker provider configuration",
    Attributes: map[string]tfsdk.Attribute{
      "spin_config": {
        Description:         "Spinnaker CLI configuration file",
        Optional:            true,
        Type:                types.StringType,
      },
      "ignore_cert_errors": {
        Description:         "Ignore certificate errors",
        Optional:            true,
        Type:                types.BoolType,
      },
      "default_headers": {
        Description:         "Additional headers to add to every request. It shall be configured as a comma separated list (e.g. key1=value1,key2=value2)",
        Optional:            true,
        Type:                types.StringType,
      },
    },
  }, nil
}

func (p *spinnakerProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
  var config providerData
  resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
  if resp.Diagnostics.HasError() {
    return
  }

  v := reflect.ValueOf(config)
  for i := 0; i < v.Type().NumField(); i++ {
    if v.Field(i).FieldByName("Unknown").Bool() {
			resp.Diagnostics.AddAttributeWarning(
        path.Root(v.Type().Field(i).Tag.Get("tfsdk")),
				"Got unknown value",
				"Can't interpolate into provider block, ignoring attribute",
			)
    }
  }

	tflog.Trace(ctx, "Creating a gate client", map[string]interface{}{
		"DefaultHeaders": config.DefaultHeaders.Value,
		"SpinConfig": config.SpinConfig.Value,
		"IgnoreCertErrors": config.IgnoreCertErrors.Value,
	})
	client, err := newSpinClient(config.DefaultHeaders.Value, config.SpinConfig.Value, config.IgnoreCertErrors.Value)
  if err != nil {
    resp.Diagnostics.AddError(
       "Error connecting to Spinnaker",
       "Failed to connect to Spinnaker Gate: " + err.Error(),
     )
     return
  }
  
  resp.DataSourceData = client
  resp.ResourceData = client
}

func (p *spinnakerProvider) Resources(ctx context.Context) []func() resource.Resource {
  return []func() resource.Resource{
    newApplicationResource,
    newProjectResource,
    newPipelineResource,
    newPipelineTemplateResource,
  }
}

func (p *spinnakerProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
  return []func() datasource.DataSource{
    newApplicationDataSource,
    newProjectDataSource,
    newPipelineDataSource,
  }
}

func newSpinClient(defaultHeaders, spinConfig string, ignoreCertErrors bool) (*gateclient.GatewayClient, error) {
  ui := output.NewUI(false, true, output.MarshalToJson, io.Discard, io.Discard)
  return gateclient.NewGateClient(ui, "", defaultHeaders, spinConfig, ignoreCertErrors, false)
}
