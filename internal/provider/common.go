package provider

import (
  "encoding/json"

  "github.com/hashicorp/terraform-plugin-framework/diag"
)

func parseJsonFromString(pipelineStr string, pipelineStruct (*map[string]interface{})) (diag diag.Diagnostics) {
  if err := json.Unmarshal([]byte(pipelineStr), pipelineStruct); err != nil {
     diag.AddError(
       "Failed to parse the JSON data",
       "Failed to parse the JSON data:" + err.Error(),
     )
  }
  return diag
}
