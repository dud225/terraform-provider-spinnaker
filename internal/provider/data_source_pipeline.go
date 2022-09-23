package provider

import (
  "context"
  "fmt"

  "github.com/dud225/terraform-provider-spinnaker/internal/api"

  "github.com/hashicorp/terraform-plugin-framework/datasource"
  "github.com/hashicorp/terraform-plugin-framework/diag"
  "github.com/hashicorp/terraform-plugin-framework/tfsdk"
  "github.com/hashicorp/terraform-plugin-framework/types"

  "github.com/spinnaker/spin/cmd/gateclient"
)

var _ datasource.DataSourceWithConfigure = (*pipelineDataSource)(nil)

type pipelineDataSource struct {
  api.Spinnaker
}

type pipelineDataSourceObject struct {
  Application   types.String    `tfsdk:"application"`
  Id            types.String    `tfsdk:"id"`
  Name          types.String    `tfsdk:"name"`
  Pipeline      types.String    `tfsdk:"pipeline"`
}

func newPipelineDataSource() datasource.DataSource {
  return &pipelineDataSource{}
}

func (r *pipelineDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
  resp.TypeName = req.ProviderTypeName + "_pipeline"
}

func (d *pipelineDataSource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
  return tfsdk.Schema{
    Description: "Get Spinnaker pipeline information",

    Attributes: map[string]tfsdk.Attribute{
      "application": {
        Description:   "Application where the pipeline is stored",
        Required:      true,
        Type:          types.StringType,
      },
      "id": {
        Description:   "Pipeline ID",
        Computed:      true,
        Type:          types.StringType,
      },
      "name": {
        Description:   "Pipeline name",
        Required:      true,
        Type:          types.StringType,
      },
      "pipeline": {
        Description:   "Actual pipeline definition stored in Spinnaker",
        Computed:      true,
        Type:          types.StringType,
      },
    },
  }, nil
}

func (d *pipelineDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *pipelineDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
  var data pipelineDataSourceObject
  resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  pipelineRes_str, pipelineRes, err := getPipelineFromSpinnaker(ctx, d.Spinnaker, data.Application.Value, data.Name.Value)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving the pipeline configuration",
      "Failed to retrieve the pipeline configuration: " + err.Error(),
    )
  }

  pipelineId, ok := pipelineRes["id"].(string)
  if !ok {
    resp.Diagnostics.AddError(
      "Unexpected data received",
      fmt.Sprintf("While getting information about a pipeline, an unexpected type (%T) was received. This is always a bug in the provider code and should be reported to the provider developers.", pipelineRes["id"]),
    )
    return
  }

  pipeline := pipelineDataSourceObject{
    Application: data.Application,
    Id: types.String{Value: pipelineId}, 
    Name: data.Name,
    Pipeline: types.String{Value: pipelineRes_str},
  }
  resp.Diagnostics.Append(resp.State.Set(ctx, &pipeline)...)
}
