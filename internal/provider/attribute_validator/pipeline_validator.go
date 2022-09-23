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

type PipelineValidator struct{}

func (pc PipelineValidator) Description(ctx context.Context) string {
  return fmt.Sprintf("Check that the pipeline definition is valid")
}

func (pc PipelineValidator) MarkdownDescription(ctx context.Context) string {
  return pc.Description(ctx)
}

type standalonePipeline struct {
  Application   string
  Name          string
  Stages    []pipelineStage
  Triggers  []pipelineTrigger
}

// https://spinnaker.io/docs/reference/pipeline/templates/
type templatedPipeline struct {
  Application   string
  Exclude       []string
  Metadata      pipelineMetadata
  Name          string
  Schema        string
  Stages        []pipelineStage
  Template      pipelineTemplateReference
  Triggers      []pipelineTrigger
  Variables     []pipelineVariable
}

type pipelineMetadata struct {
  Name          string
  Description   string
  Scopes        []string
}

type pipelineStage struct {
  Name                  string
  RefId                 string
  RequisiteStageRefIds  []string
  Type                  string
}

type pipelineTemplateReference struct {
  ArtifactAccount  string
  Reference        []string
  Type             string
}

type pipelineTrigger struct {
  Type  string
}

type pipelineVariable struct {
  Description   string
  Name          string
  Type          string
}

func (pc PipelineValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
  if req.AttributeConfig.IsUnknown() || req.AttributeConfig.IsNull() {
    return
  }

  var tfPipeline types.String
  resp.Diagnostics.Append(tfsdk.ValueAs(ctx, req.AttributeConfig, &tfPipeline)...)
	if resp.Diagnostics.HasError() {
    return
  }

  var pipeline interface{}
  var standalonePipeline standalonePipeline
  var templatedPipeline templatedPipeline
  if err := json.Unmarshal([]byte(tfPipeline.Value), &standalonePipeline); err != nil {
    if err = json.Unmarshal([]byte(tfPipeline.Value), &templatedPipeline); err != nil {
      resp.Diagnostics.AddAttributeError(
        req.AttributePath,
        "Error reading the pipeline definition",
        "Failed to read the pipeline definition: " + err.Error(),
      )
      return
    } else {
      pipeline = templatedPipeline
    }
  } else {
    pipeline = standalonePipeline
  }

  if err := ensurePipelineIsValid(reflect.ValueOf(pipeline), "pipeline.definition"); err != nil {
    resp.Diagnostics.AddAttributeError(
      req.AttributePath,
      "Invalid pipeline definition",
      "The pipeline definition is not valid: " + strings.ToLower(err.Error()),
    )
    return
  }
}

func ensurePipelineIsValid(v reflect.Value, path string) (err error) {
  switch v.Kind() {
  case reflect.Slice:
    for i := 0; i < v.Len(); i++ {
      fieldPath := fmt.Sprintf("%s[%d]", path, i)
      if v.Index(i).IsZero() {
        err = fmt.Errorf("Missing field %s", fieldPath)
        break
      }
      if err = ensurePipelineIsValid(v.Index(i), fieldPath); err != nil {
        break
      }
    }
  case reflect.Struct:
    for i := 0; i < v.NumField(); i++ {
      fieldPath := fmt.Sprintf("%s.%s", path, v.Type().Field(i).Name)
      if v.Field(i).IsZero() {
        err = fmt.Errorf("Missing field %s", fieldPath)
        break
      }
      if err = ensurePipelineIsValid(v.Field(i), fieldPath); err != nil {
        break
      }
    }
  case reflect.String:
    break
  case reflect.Invalid:
    err = fmt.Errorf("Invalid field value %s", path)
  default:
    err = fmt.Errorf("Unhandled field %s", path)
  }
  return err
}
