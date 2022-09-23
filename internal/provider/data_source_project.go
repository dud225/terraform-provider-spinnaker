package provider

import (
  "context"
  "fmt"

  "github.com/dud225/terraform-provider-spinnaker/internal/api"

  "github.com/mitchellh/mapstructure"

  "github.com/hashicorp/terraform-plugin-framework/datasource"
  "github.com/hashicorp/terraform-plugin-framework/diag"
  "github.com/hashicorp/terraform-plugin-framework/tfsdk"
  "github.com/hashicorp/terraform-plugin-framework/types"
  "github.com/hashicorp/terraform-plugin-log/tflog"

  "github.com/spinnaker/spin/cmd/gateclient"
)

var _ datasource.DataSourceWithConfigure = (*projectDataSource)(nil)

type projectDataSource struct {
  api.Spinnaker
}

func newProjectDataSource() datasource.DataSource {
  return &projectDataSource{}
}

func (r *projectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
  resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
  return tfsdk.Schema{
    Description: "Get Spinnaker project information",

    Attributes: map[string]tfsdk.Attribute{
      "config": {
        Computed:      true,
        Description:   "Project configuration",
        Type:          types.StringType,
      },
      "email": {
        Computed:      true,
        Description:   "Project owner's email",
        Type:          types.StringType,
      },
      "id": {
        Computed:      true,
        Description:   "Project ID",
        Type:          types.StringType,
      },
      "name": {
        Description:   "Project name",
        Required:      true,
        Type:          types.StringType,
      },
    },
  }, nil
}

func (d *projectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
  if req.ProviderData == nil {
    return
  }
  client, ok := req.ProviderData.(*gateclient.GatewayClient)
  if !ok {
    resp.Diagnostics.AddError(
      "Unexpected provider data type",
      fmt.Sprintf("Expected *gateclient.GatewayClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
    )
    return
  }
  d.Client = client
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
  var data project
  resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  res, err := d.GetProject(ctx, data.Name.Value)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving project information",
      "Failed to retrieve the project information: " + err.Error(),
    )
    return
  }
  tflog.Trace(ctx, "Retrieved project information", map[string]interface{}{
    "payload": res,
  })

  project := project{Name: data.Name}
  msd, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
    DecodeHook: projectDecodeHookFunc(ctx),
    Result:     &project,
  })
  if err != nil {
    resp.Diagnostics.AddError(
      "Failed to create a mapstructure decoder",
      "Failed to create a mapstructure decoder: " + err.Error(),
    )
    return
  }
  if err := msd.Decode(res); err != nil {
    resp.Diagnostics.AddError(
      "Failed to decode the data",
      "Failed to decode the data received from Spinnaker: " + err.Error(),
    )
    return
  }

  resp.Diagnostics.Append(resp.State.Set(ctx, &project)...)
}
