package provider

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/dud225/terraform-provider-spinnaker/internal/api"
  "github.com/dud225/terraform-provider-spinnaker/internal/provider/attribute_validator"

  "github.com/hashicorp/terraform-plugin-framework/diag"
  "github.com/hashicorp/terraform-plugin-framework/path"
  "github.com/hashicorp/terraform-plugin-framework/resource"
  "github.com/hashicorp/terraform-plugin-framework/tfsdk"
  "github.com/hashicorp/terraform-plugin-framework/types"
  "github.com/hashicorp/terraform-plugin-log/tflog"

  "github.com/spinnaker/spin/cmd/gateclient"
)

var _ resource.ResourceWithConfigure = (*pipelineResource)(nil)
var _ resource.ResourceWithImportState = (*pipelineResource)(nil)

type pipelineResource struct {
  api.Spinnaker
}

type pipelineResourceObject struct {
  Id         types.String     `tfsdk:"id"`
  Pipeline   pipelineDetails  `tfsdk:"pipeline"`
}

type pipelineDetails struct {
  Definition  types.String  `tfsdk:"definition"`
  Result      types.String  `tfsdk:"result"`
}

func newPipelineResource() resource.Resource {
  return &pipelineResource{}
}

func (r *pipelineResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
  resp.TypeName = req.ProviderTypeName + "_pipeline"
}

func (r *pipelineResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
  return tfsdk.Schema{
    Description: "Manage Spinnaker pipeline",

    Attributes: map[string]tfsdk.Attribute{
      "id": {
        Computed:      true,
        Description:   "Pipeline ID",
        PlanModifiers: tfsdk.AttributePlanModifiers{
          resource.UseStateForUnknown(),
        },
        Type:          types.StringType,
      },
      "pipeline": {
        Required:      true,
        Attributes:    tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
          "definition": {
            Description:   "Pipeline definition",
            Required:      true,
            Type:          types.StringType,
            Validators:    []tfsdk.AttributeValidator{
              attribute_validator.PipelineValidator{},
            },
          },
          /*
            Spinnaker adds a bunch of fields on a pipeline
            see for a subset of those: https://github.com/armory-io/terraform-provider-spinnaker/blob/34a9104010870b549824a8a5c8b0a287bc655772/spinnaker/resource_pipeline.go#L195-L202
            as a result the pipeline that is stored on Spinnaker is different that the one being used to create it
            but Terraform doesn't endorse such usage: "Error: Provider produced inconsistent result after apply"
            so we have to manage those data in a separate field
          */
          "result": {
            Computed:      true,
            Description:   "Actual pipeline definition stored in Spinnaker",
            Type:          types.StringType,
          },
        }),
      },
    },
  }, nil
}

func (r *pipelineResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *pipelineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
  var data pipelineResourceObject
  resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  pipelineDef := make(map[string]interface{})
  resp.Diagnostics.Append(parseJsonFromString(data.Pipeline.Definition.Value, &pipelineDef)...)
  if resp.Diagnostics.HasError() {
    return
  }
  
  if template, exists := pipelineDef["template"]; exists && len(template.(map[string]interface{})) > 0 {
    pipelineDef["type"] = "templatedPipeline"
  }

  tflog.Trace(ctx, "Creating a pipeline", map[string]interface{}{
    "payload": pipelineDef,
  })
  if err := r.CreatePipeline(ctx, pipelineDef); err != nil {
     resp.Diagnostics.AddError(
       "Error creating a pipeline",
       "Failed to create a pipeline: " + err.Error(),
     )
     return
  }

  pipelineRes_str, pipelineRes, err := getPipelineFromSpinnaker(ctx, r.Spinnaker, pipelineDef["application"].(string), pipelineDef["name"].(string))
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

  pipeline := pipelineResourceObject{
    Id: types.String{Value: pipelineId}, 
    Pipeline: pipelineDetails{
      Definition: data.Pipeline.Definition,
      Result: types.String{Value: pipelineRes_str},
    },
  }
  resp.Diagnostics.Append(resp.State.Set(ctx, &pipeline)...)
}

func (r *pipelineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
  var data pipelineResourceObject
  resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

	var pipelineDef_str, pipelineRes_str string
	/*
		When importing a pipeline template, the definition attribute is empty, the practioner only provides the ID
		so to fill it in we use the actual pipeline template stored in Spinnaker.
	*/
	if data.Pipeline.Definition.Null {
		applicationName, pipelineName, err := getPipelineMetadata(ctx, r.Spinnaker, data.Id.Value)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error retrieving the pipeline metadata",
				"Failed to retrieve the pipeline metadata: " + err.Error(),
			)
		}

		pipelineDef_str, _, err = getPipelineFromSpinnaker(ctx, r.Spinnaker, applicationName, pipelineName)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error retrieving the pipeline configuration",
				"Failed to retrieve the pipeline configuration: " + err.Error(),
			)
		}
	} else {
		var pipelineDef, pipelineRes map[string]interface{}
		resp.Diagnostics.Append(parseJsonFromString(data.Pipeline.Definition.Value, &pipelineDef)...)
		if resp.Diagnostics.HasError() {
			return
		}
		
		applicationName, pipelineName, err := getPipelineMetadata(ctx, r.Spinnaker, data.Id.Value)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error retrieving the pipeline metadata",
				"Failed to retrieve the pipeline metadata: " + err.Error(),
			)
		}

		pipelineRes_str, pipelineRes, err = getPipelineFromSpinnaker(ctx, r.Spinnaker, applicationName, pipelineName)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error retrieving the pipeline configuration",
				"Failed to retrieve the pipeline configuration: " + err.Error(),
			)
		}

		// Synchronize the definition field
		for key := range pipelineDef {
			value, ok := pipelineRes[key]
			if ok {
				pipelineDef[key] = value
			}
		}
		pipelineDefUpdated, err := json.Marshal(pipelineDef)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error building the updated pipeline definition",
				"Failed to build the updated pipeline definition: " + err.Error(),
			)
		}
		pipelineDef_str = string(pipelineDefUpdated)
	}

  pipeline := pipelineResourceObject{
    Id: data.Id,
    Pipeline: pipelineDetails{
      Definition: types.String{Value: pipelineDef_str},
      Result: types.String{Value: pipelineRes_str},
    },
  }
  resp.Diagnostics.Append(resp.State.Set(ctx, &pipeline)...)
}

func (r *pipelineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
  var data pipelineResourceObject
  resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  pipelineDef := make(map[string]interface{})
  resp.Diagnostics.Append(parseJsonFromString(data.Pipeline.Definition.Value, &pipelineDef)...)
  if resp.Diagnostics.HasError() {
    return
  }

  pipelineDef["id"] = data.Id.Value
  if template, exists := pipelineDef["template"]; exists && len(template.(map[string]interface{})) > 0 {
    pipelineDef["type"] = "templatedPipeline"
  }

  tflog.Trace(ctx, "Updating a pipeline", map[string]interface{}{
    "payload": pipelineDef,
  })
  if err := r.CreatePipeline(ctx, pipelineDef); err != nil {
     resp.Diagnostics.AddError(
       "Error updating a pipeline",
       "Failed to update a pipeline: " + err.Error(),
     )
     return
  }

  pipelineRes, _, err := getPipelineFromSpinnaker(ctx, r.Spinnaker, pipelineDef["application"].(string), pipelineDef["name"].(string))
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving the pipeline configuration",
      "Failed to retrieve the pipeline configuration: " + err.Error(),
    )
  }

  pipeline := pipelineResourceObject{
    Id: data.Id,
    Pipeline: pipelineDetails{
      Definition: data.Pipeline.Definition,
      Result: types.String{Value: pipelineRes},
    },
  }
  resp.Diagnostics.Append(resp.State.Set(ctx, &pipeline)...)
}

func (r *pipelineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
  var data pipelineResourceObject
  resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  /*
    Even though we already know the application and the pipeline names from state,
    the ID is what serves as the reference for the updates
    so I think it's more accurate to delete the pipeline that matches a given ID
  */
  applicationName, pipelineName, err := getPipelineMetadata(ctx, r.Spinnaker, data.Id.Value)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving the pipeline metadata",
      "Failed to retrieve the pipeline metadata: " + err.Error(),
    )
  }

  tflog.Trace(ctx, "Deleting a pipeline", map[string]interface{}{
    "pipelineId": data.Id.Value,
    "application": applicationName,
    "pipeline": pipelineName,
  })
  if err := r.DeletePipeline(ctx, applicationName, pipelineName); err != nil {
    resp.Diagnostics.AddError(
      "Error deleting pipeline",
      "Failed to delete the pipeline: " + err.Error(),
    )
    return
  }
}

func (r *pipelineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
  /*
		The definition att
  applicationName, pipelineName, err := getPipelineMetadata(ctx, r.Spinnaker, req.ID)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving the pipeline metadata",
      "Failed to retrieve the pipeline metadata: " + err.Error(),
    )
  }

  pipelineRes_str, _, err := getPipelineFromSpinnaker(ctx, r.Spinnaker, applicationName, pipelineName)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving the pipeline configuration",
      "Failed to retrieve the pipeline configuration: " + err.Error(),
    )
  }

  resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("pipeline").AtName("definition"), pipelineRes_str)...)
	*/
  resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getPipelineMetadata(ctx context.Context, api api.Spinnaker, pipelineId string) (string, string, error) {
  tflog.Trace(ctx, "Getting pipeline metadata", map[string]interface{}{
    "pipelineId": pipelineId,
  })
  applicationList, err := api.ListApplications(ctx)
  if err != nil {
    return "", "", fmt.Errorf("Failed to retrieve the application list: %w", err)
  }

  for _, appInfo := range applicationList {
    applicationName, ok := appInfo["name"].(string)
    if !ok {
      return "", "", fmt.Errorf("While getting information about an application, an unexpected type (%T) was received. " +
                                "This is always a bug in the provider code and should be reported to the provider developers.", appInfo["name"])
    }

    pipelineList, err := api.ListPipelines(ctx, applicationName)
    if err != nil {
      return "", "", fmt.Errorf("Failed to retrieve the pipeline list: %w", err)
    }
    for _, pipelineInfo := range pipelineList {
      id, ok := pipelineInfo["id"].(string)
      if !ok {
        return "", "", fmt.Errorf("While getting information about an application, an unexpected type (%T) was received. " + 
                                  "This is always a bug in the provider code and should be reported to the provider developers.", pipelineInfo["id"])
      }
      if pipelineId == id {
        pipelineName, ok := pipelineInfo["name"].(string)
        if !ok {
          return "", "", fmt.Errorf("While getting information about an application, an unexpected type (%T) was received. " +
                                  "This is always a bug in the provider code and should be reported to the provider developers.", pipelineInfo["name"])
        }
        return applicationName, pipelineName, nil
      }
    }
  } 

  return "", "", fmt.Errorf("Pipeline not found")
}

func getPipelineFromSpinnaker(ctx context.Context, api api.Spinnaker, applicationName, pipelineName string) (string, map[string]interface{}, error) {
  res, err := api.GetPipeline(ctx, applicationName, pipelineName)
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
