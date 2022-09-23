package attribute_validator

import (
  "context"
  "encoding/json"
  "fmt"
  "reflect"
  "strings"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type PipelineTemplateValidator struct{}

func (pc PipelineTemplateValidator) Description(ctx context.Context) string {
  return fmt.Sprintf("Check that the pipeline definition is valid")
}

func (pc PipelineTemplateValidator) MarkdownDescription(ctx context.Context) string {
  return pc.Description(ctx)
}

// https://spinnaker.io/docs/reference/pipeline/templates/
type pipelineTemplate struct {
  Id        string
  Metadata  pipelineTemplateMetadata
  Pipeline  pipelineTemplatePipeline
  Schema    string
  // Variables is not mandatory
}

type pipelineTemplateMetadata struct {
  Name          string
  Description   string
  Owner         string
  Scopes        []string
}

type pipelineTemplatePipeline struct {
  Stages    []pipelineStage
  Triggers  []pipelineTrigger
}

func (pc PipelineTemplateValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
  if req.AttributeConfig.IsUnknown() || req.AttributeConfig.IsNull() {
    return
  }

  var tfPipelineTemplate types.String
  resp.Diagnostics.Append(tfsdk.ValueAs(ctx, req.AttributeConfig, &tfPipelineTemplate)...)
	if resp.Diagnostics.HasError() {
    return
  }

  var pipelineTemplate pipelineTemplate
  if err := json.Unmarshal([]byte(tfPipelineTemplate.Value), &pipelineTemplate); err != nil {
    resp.Diagnostics.AddAttributeError(
      req.AttributePath,
      "Error reading the pipeline template definition",
      "Failed to read the pipeline template definition: " + err.Error(),
    )
  }

  if err := ensurePipelineTemplateIsValid(reflect.ValueOf(pipelineTemplate), "pipeline_template"); err != nil {
    resp.Diagnostics.AddAttributeError(
      req.AttributePath,
      "Invalid pipeline template template definition",
      "The pipeline template definition is not valid: " + strings.ToLower(err.Error()),
    )
    return
  }
}

func ensurePipelineTemplateIsValid(v reflect.Value, path string) (err error) {
  return ensurePipelineIsValid(v, path)
}
