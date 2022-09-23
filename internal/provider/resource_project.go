package provider

import (
  "context"
  "encoding/json"
  "fmt"
  "reflect"

  "github.com/dud225/terraform-provider-spinnaker/internal/api"
  "github.com/dud225/terraform-provider-spinnaker/internal/provider/attribute_plan_modifier"
  "github.com/dud225/terraform-provider-spinnaker/internal/provider/attribute_validator"

  "github.com/mitchellh/mapstructure"

  "github.com/hashicorp/terraform-plugin-framework/diag"
  "github.com/hashicorp/terraform-plugin-framework/path"
  "github.com/hashicorp/terraform-plugin-framework/resource"
  "github.com/hashicorp/terraform-plugin-framework/tfsdk"
  "github.com/hashicorp/terraform-plugin-framework/types"
  "github.com/hashicorp/terraform-plugin-log/tflog"

  "github.com/spinnaker/spin/cmd/gateclient"
)

var _ resource.ResourceWithConfigure = (*projectResource)(nil)
var _ resource.ResourceWithImportState = (*projectResource)(nil)

type projectResource struct {
  api.Spinnaker
}

type project struct {
  Config  types.String  `tfsdk:"config"`
  Email   types.String  `tfsdk:"email"`
  Id      types.String  `tfsdk:"id"`
  Name    types.String  `tfsdk:"name"`
}

func newProjectResource() resource.Resource {
  return &projectResource{}
}

func (r *projectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
  resp.TypeName = req.ProviderTypeName + "_project"
}

func projectDefaultConfig() (string, error) {
  defaultConfig := map[string]interface{}{
    "applications": make([]string, 0),
    "clusters": make([]map[string]interface{}, 0),
    "pipelineConfigs": make([]map[string]interface{}, 0),
  }
  defaultConfigBytes, err := json.Marshal(defaultConfig)
  return string(defaultConfigBytes), err  
}

func (r *projectResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
  var diag diag.Diagnostics
  defaultConfig, err := projectDefaultConfig()
  if err != nil {
    diag.AddError(
      "Error building the project default config",
      "Failed to build the project default config: " + err.Error(),
    )
  }
  return tfsdk.Schema{
    Description: "Manage Spinnaker project",

    Attributes: map[string]tfsdk.Attribute{
      "config": {
        Computed:      true,
        Description:   fmt.Sprintf("Project configuration. Default value: %s", defaultConfig),
        Optional:      true,
        Type:          types.StringType,
        Validators:    []tfsdk.AttributeValidator{
          attribute_validator.ProjectConfigValidator{},
        },
        PlanModifiers: []tfsdk.AttributePlanModifier{
          attribute_plan_modifier.DefaultValue(types.String{Value: defaultConfig}),
        },
      },
      "email": {
        Description: 	 "Project owner's email",
        Required:      true,
        Type:          types.StringType,
        Validators:    []tfsdk.AttributeValidator{
          attribute_validator.EmailValidator{},
        },
      },
      "id": {
        Computed:      true,
        Description:   "Project ID",
        PlanModifiers: tfsdk.AttributePlanModifiers{
          resource.UseStateForUnknown(),
        },
        Type:          types.StringType,
      },
      "name": {
        Description:   "Project name",
        Required:      true,
        Type:          types.StringType,
      },
    },
  }, diag
}

func (r *projectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
  r.Client = client
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
  var data project
  resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  config := make(map[string]interface{})
  if err := json.Unmarshal([]byte(data.Config.Value), &config); err != nil {
     resp.Diagnostics.AddError(
       "Error reading the project config",
       "Failed to read the project config: " + err.Error(),
     )
     return
  }
  project := map[string]interface{}{
    "config": config,
    "name": data.Name.Value,
    "email": data.Email.Value,
  }

  tflog.Trace(ctx, "Creating a project", map[string]interface{}{
    "payload": project,
  })
  if err := r.CreateProject(ctx, project); err != nil {
     resp.Diagnostics.AddError(
       "Error creating a project",
       "Failed to create a project: " + err.Error(),
     )
     return
  }

  res, err := r.GetProject(ctx, data.Name.Value)
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
  projectId, ok := res["id"].(string)
  if !ok {
    resp.Diagnostics.AddError(
      "Unexpected data received",
      fmt.Sprintf("While getting information about a project, an unexpected type (%T) was received. This is always a bug in the provider code and should be reported to the provider developers.", res),
    )
    return
  }

  data.Id = types.String{Value: projectId}
  resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
  var data project
  resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  res, err := r.GetProject(ctx, data.Id.Value)
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

  project := project{Id: data.Id}
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

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
  var data project
  resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  config := make(map[string]interface{})
  if err := json.Unmarshal([]byte(data.Config.Value), &config); err != nil {
     resp.Diagnostics.AddError(
       "Error reading the project config",
       "Failed to read the project config: " + err.Error(),
     )
     return
  }
  project := map[string]interface{}{
    "config": config,
    "id": data.Id.Value,
    "name": data.Name.Value,
    "email": data.Email.Value,
  }

  tflog.Trace(ctx, "Updating a project", map[string]interface{}{
    "payload": project,
  })
  if err := r.CreateProject(ctx, project); err != nil {
     resp.Diagnostics.AddError(
       "Error updating a project",
       "Failed to update a project: " + err.Error(),
     )
     return
  }

  resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
  var data project
  resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  tflog.Trace(ctx, "Deleting a project", map[string]interface{}{
    "projectId": data.Id.Value,
  })
  if err := r.DeleteProject(ctx, data.Id.Value); err != nil {
    resp.Diagnostics.AddError(
      "Error deleting project",
      "Failed to delete the project: " + err.Error(),
    )
    return
  }
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
  resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func projectDecodeHookFunc(ctx context.Context) mapstructure.DecodeHookFuncType {
  return mapstructure.DecodeHookFuncType(func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
    tflog.Trace(ctx, "Executing function projectDecodeHookFunc", map[string]interface{}{
      "from": from.String(),
      "to": to.String(),
      "data": data,
    })

    // Config field
    if from == reflect.TypeOf(map[string]interface{}(nil)) && to == reflect.TypeOf(types.String{}) {
      config, ok := data.(map[string]interface{})
      if !ok {
        return nil, fmt.Errorf("projectDecodeHookFunc: unexpected data received: %s (type: %T)", data, data)
      }

      configStr, err := json.Marshal(config)
      return types.String{Value: string(configStr)}, err

    // All other fields
    } else if to == reflect.TypeOf(types.String{}) {
      dataStr, ok := data.(string)
      if !ok {
        return nil, fmt.Errorf("projectDecodeHookFunc: unexpected data received: %s (type: %T)", data, data)
      }
      return types.String{Value: dataStr}, nil
    }

    return data, nil
  })
}
