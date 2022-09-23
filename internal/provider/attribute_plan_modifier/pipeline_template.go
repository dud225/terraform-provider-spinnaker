package attribute_plan_modifier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
  "github.com/hashicorp/terraform-plugin-framework/types"
)

type PipelineTemplateConditionalReplace struct {}

func (p *PipelineTemplateConditionalReplace) Description() string {
	return fmt.Sprintf("Recreates the pipeline template if the ID has changed")
}

func (p *PipelineTemplateConditionalReplace) MarkdownDescription() string {
	return p.Description()
}

func (p *PipelineTemplateConditionalReplace) Check(ctx context.Context, state, config attr.Value, path path.Path) (bool, diag.Diagnostics) {
	var diag diag.Diagnostics
	var configPipelineTemplate_str, statePipelineTemplate_str string
	diag.Append(tfsdk.ValueAs(ctx, config, &configPipelineTemplate_str)...)
	diag.Append(tfsdk.ValueAs(ctx, state, &statePipelineTemplate_str)...)
	if diag.HasError() {
		return false, diag
	}

  var configPipelineTemplate, statePipelineTemplate map[string]interface{}
  if err := json.Unmarshal([]byte(configPipelineTemplate_str), &configPipelineTemplate); err != nil {
		diag.AddAttributeError(
			path,
		  "Failed to parse the JSON config data",
      "Failed to parse the JSON config data, error:" + err.Error(),
		)
  }
  if err := json.Unmarshal([]byte(statePipelineTemplate_str), &statePipelineTemplate); err != nil {
		diag.AddAttributeError(
			path,
      "Failed to parse the JSON state data",
      "Failed to parse the JSON state data, error:" + err.Error(),
		)
  }
	if diag.HasError() {
		return false, diag
	}

	configPipelineTemplateId, ok := configPipelineTemplate["id"]
	if !ok {
		diag.AddAttributeError(
			path,
      "Unexpected config type",
      fmt.Sprintf("While getting information about an application, an unexpected type (%T) was received. " +
								 "This is always a bug in the provider code and should be reported to the provider developers.", configPipelineTemplate["id"]),
		)
	}
	statePipelineTemplateId, ok := statePipelineTemplate["id"]
	if !ok {
		diag.AddAttributeError(
			path,
      "Unexpected state type",
      fmt.Sprintf("While getting information about an application, an unexpected type (%T) was received. " +
								 "This is always a bug in the provider code and should be reported to the provider developers.", statePipelineTemplate["id"]),
		)
	}
	if diag.HasError() {
		return false, diag
	}

	// If the pipeline template ID is changing, then recreate it
	if configPipelineTemplateId != statePipelineTemplateId {
		return true, diag
	}

	return false, diag
}

var _ tfsdk.AttributePlanModifier = (*pipelineTemplateSanitize)(nil)

type pipelineTemplateSanitize struct {
	fieldsManagedBySpinnaker []string
}

func PipelineTemplateSanitize() tfsdk.AttributePlanModifier {
	return &pipelineTemplateSanitize{[]string{"updateTs"}}
}

func (pts *pipelineTemplateSanitize) Description(_ context.Context) string {
	return fmt.Sprintf("Strip out fields managed by Spinnaker")
}

func (pts *pipelineTemplateSanitize) MarkdownDescription(_ context.Context) string {
	return pts.Description(nil)
}

func (pts *pipelineTemplateSanitize) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, res *tfsdk.ModifyAttributePlanResponse) {
	var pipelineTemplate_str string
	res.Diagnostics.Append(tfsdk.ValueAs(ctx, res.AttributePlan, &pipelineTemplate_str)...)
	if res.Diagnostics.HasError() {
		return
	}

	var pipelineTemplate map[string]interface{}
  if err := json.Unmarshal([]byte(pipelineTemplate_str), &pipelineTemplate); err != nil {
    res.Diagnostics.AddAttributeError(
      req.AttributePath,
      "Error reading the pipeline template definition",
      "Failed to read the pipeline template definition: " + err.Error(),
    )
  }
	if res.Diagnostics.HasError() {
		return
	}

	for _, field := range pts.fieldsManagedBySpinnaker {
		delete(pipelineTemplate, field)
	}

  pipelineTemplateSanitized, err := json.Marshal(pipelineTemplate)
  if err != nil {
    res.Diagnostics.AddError(
      "Error building the updated pipeline definition",
      "Failed to build the updated pipeline definition: " + err.Error(),
    )
  }
	res.AttributePlan = types.String{Value: string(pipelineTemplateSanitized)}
}
