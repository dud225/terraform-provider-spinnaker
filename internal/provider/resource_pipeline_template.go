package provider

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/dud225/terraform-provider-spinnaker/internal/api"
  "github.com/dud225/terraform-provider-spinnaker/internal/provider/attribute_plan_modifier"
  "github.com/dud225/terraform-provider-spinnaker/internal/provider/attribute_validator"

  "github.com/hashicorp/terraform-plugin-framework/diag"
  "github.com/hashicorp/terraform-plugin-framework/path"
  "github.com/hashicorp/terraform-plugin-framework/resource"
  "github.com/hashicorp/terraform-plugin-framework/tfsdk"
  "github.com/hashicorp/terraform-plugin-framework/types"
  "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
  "github.com/hashicorp/terraform-plugin-log/tflog"

  "github.com/spinnaker/spin/cmd/gateclient"
)

var _ resource.ResourceWithConfigure = (*pipelineTemplateResource)(nil)
var _ resource.ResourceWithImportState = (*pipelineTemplateResource)(nil)

type pipelineTemplateResource struct {
  api.Spinnaker
}

type pipelineTemplate struct {
  Id                types.String  					`tfsdk:"id"`
  PipelineTemplate  pipelineTemplateDetails	`tfsdk:"pipeline_template"`
	Tag								types.String						`tfsdk:"tag"`
}

type pipelineTemplateDetails struct {
  Definition  types.String  `tfsdk:"definition"`
  Result      types.String  `tfsdk:"result"`
}

func newPipelineTemplateResource() resource.Resource {
  return &pipelineTemplateResource{}
}

func (r *pipelineTemplateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
  resp.TypeName = req.ProviderTypeName + "_pipeline_template"
}

func (r *pipelineTemplateResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	pipelineTemplateConditionalReplace := (*attribute_plan_modifier.PipelineTemplateConditionalReplace)(nil)
  return tfsdk.Schema{
    Description: "Manage Spinnaker pipeline template",

    Attributes: map[string]tfsdk.Attribute{
      "id": {
        Computed:      true,
        Description:   "Pipeline template ID",
        PlanModifiers: tfsdk.AttributePlanModifiers{
          resource.UseStateForUnknown(),
        },
        Type:          types.StringType,
      },
      "pipeline_template": {
        Required:      true,
        Attributes:    tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
          "definition": {
            Description:   "Pipeline template definition",
            Required:      true,
            Type:          types.StringType,
						PlanModifiers:  tfsdk.AttributePlanModifiers{
							resource.RequiresReplaceIf(
								pipelineTemplateConditionalReplace.Check,
								pipelineTemplateConditionalReplace.Description(),
								pipelineTemplateConditionalReplace.MarkdownDescription(),
							),
						},
						Validators:    []tfsdk.AttributeValidator{
							attribute_validator.PipelineTemplateValidator{},
						},
          },
          "result": {
            Computed:      true,
            Description:   "Actual pipeline template definition stored in Spinnaker",
            Type:          types.StringType,
          },
        }),
      },
      "tag": {
        Description:    "Pipeline template tag",
        Optional:       true,
        Type:           types.StringType,
        Validators:     []tfsdk.AttributeValidator{
          // See supported tags:
          // https://github.com/spinnaker/front50/blob/e6a4399e1b9898db19f3ed961fc1d4c5f61ec784/front50-web/src/main/java/com/netflix/spinnaker/front50/controllers/V2PipelineTemplateController.java
          stringvalidator.OneOf([]string{"latest", "stable", "unstable", "experimental", "test", "canary"}...),
        },
      },
    },
  }, nil
}

func (r *pipelineTemplateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *pipelineTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
  var data pipelineTemplate
  resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  pipelineTemplateDef := make(map[string]interface{})
  resp.Diagnostics.Append(parseJsonFromString(data.PipelineTemplate.Definition.Value, &pipelineTemplateDef)...)
  if resp.Diagnostics.HasError() {
    return
  }
  
	id, ok := pipelineTemplateDef["id"].(string)
	if !ok {
    resp.Diagnostics.AddAttributeError(
      path.Root("pipeline_template"),
      "Missing pipeline template ID",
      "Missing pipeline template ID",
     )
  }
	var tag string
	if !data.Tag.Null {
		tag = data.Tag.Value
	}
  tflog.Trace(ctx, "Creating a pipeline template", map[string]interface{}{
    "payload": pipelineTemplateDef,
		"tag": tag,
  })
  if err := r.CreatePipelineTemplate(ctx, pipelineTemplateDef, tag); err != nil {
    resp.Diagnostics.AddError(
      "Error creating a pipeline template",
      "Failed to create a pipeline template: " + err.Error(),
     )
     return
  }

  pipelineTemplateRes, err := r.GetPipelineTemplate(ctx, id, tag)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving the pipeline metadata",
      "Failed to retrieve the pipeline metadata: " + err.Error(),
    )
  }

  pipelineTemplate := pipelineTemplateResourceObject{
    Id: types.String{Value: id}, 
    Pipeline: pipelineDetails{
      Definition: data.PipelineTemplate.Definition,
      Result: types.String{Value: pipelineTemplateRes_str},
    },
  }
  resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *pipelineTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
  var data pipelineTemplate
  resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  pipelineTemplateDef := make(map[string]interface{})
  resp.Diagnostics.Append(parseJsonFromString(data.PipelineTemplate.Value, &pipelineTemplateDef)...)
  if resp.Diagnostics.HasError() {
    return
  }
  
  res, err := r.GetPipelineTemplate(ctx, data.Id.Value, data.Tag.Value)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving the pipeline metadata",
      "Failed to retrieve the pipeline metadata: " + err.Error(),
    )
  }

	// Strip out fields managed by Spinnaker to avoid a constant diff with the config
	for _, field := range []string{"lastModifiedBy", "updateTs"} {
		delete(res, field)
	}

  pipelineTemplateUpdated, err := json.Marshal(res)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error building the updated pipeline definition",
      "Failed to build the updated pipeline definition: " + err.Error(),
    )
  }

	data.PipelineTemplate = types.String{Value: string(pipelineTemplateUpdated)}
  resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *pipelineTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
  var data pipelineTemplate
  resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  pipelineTemplate := make(map[string]interface{})
  resp.Diagnostics.Append(parseJsonFromString(data.PipelineTemplate.Value, &pipelineTemplate)...)
  if resp.Diagnostics.HasError() {
    return
  }
  
	var tag string
	if !data.Tag.Null {
		tag = data.Tag.Value
	}
  tflog.Trace(ctx, "Updating a pipeline template", map[string]interface{}{
    "payload": pipelineTemplate,
		"pipelineTemplateId": data.Id.Value,
		"tag": tag,
  })
  if err := r.UpdatePipelineTemplate(ctx, data.Id.Value, pipelineTemplate, tag); err != nil {
    resp.Diagnostics.AddError(
      "Error creating a pipeline template",
      "Failed to create a pipeline template: " + err.Error(),
     )
     return
  }

  resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *pipelineTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
  var data pipelineTemplate
  resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

	var tag string
	if !data.Tag.Null {
		tag = data.Tag.Value
	}
  tflog.Trace(ctx, "Deleting a pipeline", map[string]interface{}{
    "pipelineTemplateId": data.Id.Value,
		"tag": tag,
  })
  if err := r.DeletePipelineTemplate(ctx, data.Id.Value, tag); err != nil {
    resp.Diagnostics.AddError(
      "Error deleting pipeline",
      "Failed to delete the pipeline: " + err.Error(),
    )
    return
  }
}

func (r *pipelineTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
  resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getPipelineTemplateFromSpinnaker(ctx context.Context, api api.Spinnaker, pipelineTemplateId, tag string) (string, map[string]interface{}, error) {
  res, err := api.GetPipelineTemplate(ctx, id, tag)
  if err != nil {
    return "", nil, fmt.Errorf("Failed to retrieve the pipeline information: %w", err)
  }
  tflog.Trace(ctx, "Retrieved pipeline information", map[string]interface{}{
    "payload": res,
  })
  pipeline, err := json.Marshal(res)
  if err != nil {
    return "", nil, fmt.Errorf("Failed to decode the data received from Spinnaker:  %w", err)
  }

  return string(pipeline), res, nil
}
