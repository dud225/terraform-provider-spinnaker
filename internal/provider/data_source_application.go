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

var _ datasource.DataSourceWithConfigure = (*applicationDataSource)(nil)

type applicationDataSource struct {
  api.Spinnaker
}

func newApplicationDataSource() datasource.DataSource {
  return &applicationDataSource{}
}

func (r *applicationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
  resp.TypeName = req.ProviderTypeName + "_application"
}

func (d *applicationDataSource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
  return tfsdk.Schema{
    Description: "Get Spinnaker application information",

    Attributes: map[string]tfsdk.Attribute{
      "cloud_providers": {
        Computed:     true,
        Description:  "List of the cloud providers used by the application",
        Type:         types.ListType{
          ElemType: types.StringType,
        },
      },
      "email": {
        Computed:     true,
        Description:  "Application owner's email",
        Type:         types.StringType,
      },
      "id": {
        Computed:     true,
        Description:  "Application name",
        Type:         types.StringType,
      },
      "name": {
        Required:     true,
        Description:  "Application name",
        Type:         types.StringType,
      },
      "permissions": {
        Computed:     true,
        Description:  "Permissions to set on the application. It should be set as a map where the key indicates the role and the value the permissions to grant",
        Type:         types.MapType{
          ElemType:   types.SetType{
            ElemType: types.StringType,
          },
        },
      },
    },
  }, nil
}

func (d *applicationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *applicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
  var data application
  resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  res, err := d.GetApplication(ctx, data.Name.Value)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving application information",
      "Failed to retrieve the application information: " + err.Error(),
    )
    return
  }
  tflog.Trace(ctx, "Retrieved application information", map[string]interface{}{
    "payload": res,
  })

  app := application{Id: data.Name}
  msd, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
    DecodeHook: applicationDecodeHookFunc(ctx),
    Result:     &app,
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

  resp.Diagnostics.Append(resp.State.Set(ctx, &app)...)
}
