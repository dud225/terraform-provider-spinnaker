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

type ProjectConfigValidator struct{}

func (pc ProjectConfigValidator) Description(ctx context.Context) string {
  return fmt.Sprintf("Check that the project config is valid")
}

func (pc ProjectConfigValidator) MarkdownDescription(ctx context.Context) string {
  return pc.Description(ctx)
}

type projectConfig struct {
  Applications    []string
  Clusters        []projectCluster
  PipelineConfigs []pipelineConfig
}

type projectCluster struct {
  Account       string
  Applications  []string
  Detail        string
  Stack         string
}

type pipelineConfig struct {
  Application       string
  PipelineConfigId  string
}

func (pc ProjectConfigValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
  if req.AttributeConfig.IsUnknown() || req.AttributeConfig.IsNull() {
    return
  }

  var tfConfig types.String
  resp.Diagnostics.Append(tfsdk.ValueAs(ctx, req.AttributeConfig, &tfConfig)...)
	if resp.Diagnostics.HasError() {
    return
  }

  var config projectConfig
  if err := json.Unmarshal([]byte(tfConfig.Value), &config); err != nil {
		resp.Diagnostics.AddAttributeError(
      req.AttributePath,
      "Error reading the project config",
      "Failed to read the project config: " + err.Error(),
    )
    return
  }

  if err := ensureProjectConfigIsValid(reflect.ValueOf(config), "config"); err != nil {
    resp.Diagnostics.AddAttributeError(
      req.AttributePath,
      "Invalid project configuration",
      "The project configuration is not valid: " + strings.ToLower(err.Error()),
    )
    return
  }
}

func ensureProjectConfigIsValid(v reflect.Value, path string) (err error) {
  switch v.Kind() {
  case reflect.Slice:
    for i := 0; i < v.Len(); i++ {
      fieldPath := fmt.Sprintf("%s[%d]", path, i)
      if v.Index(i).IsZero() {
        err = fmt.Errorf("Missing field %s", fieldPath)
        break
      }
      if err = ensureProjectConfigIsValid(v.Index(i), fieldPath); err != nil {
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
      if err = ensureProjectConfigIsValid(v.Field(i), fieldPath); err != nil {
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
