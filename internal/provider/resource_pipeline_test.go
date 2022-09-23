package provider

import (
  "context"
  "encoding/json"
  "fmt"
  "reflect"
  "regexp"
  "testing"

  "github.com/dud225/terraform-provider-spinnaker/internal/api"

  "github.com/hashicorp/terraform-plugin-framework/diag"
  "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
  "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
  "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const tfPipelineResourceReference = "spinnaker_pipeline.test"

func TestAccPipelineResource_basic(t *testing.T) {
  name := acctest.RandomWithPrefix("tf-acc-test")
  pipelineDef := generatePipelineConfig_basic(name, name, true)
  nameUpdated := acctest.RandomWithPrefix("tf-acc-test")
  pipelineDefUpdated := generatePipelineConfig_basic(name, nameUpdated, true)

  resource.ParallelTest(t, resource.TestCase{
    ProtoV6ProviderFactories: protoV6ProviderFactories,
    Steps: []resource.TestStep{
      {
        Config: testAccPipelineResourceConfig_basic(name, name),
        Check:  resource.ComposeAggregateTestCheckFunc(
          checkPipelineExists(),
					resource.TestCheckResourceAttr(tfPipelineResourceReference, "pipeline.definition", pipelineDef),
          checkPipelineAttributes(pipelineDef),
        ),
      },
      {
        Config: testAccPipelineResourceConfig_basic(name, nameUpdated),
        Check:  resource.ComposeAggregateTestCheckFunc(
          checkPipelineExists(),
					resource.TestCheckResourceAttr(tfPipelineResourceReference, "pipeline.definition", pipelineDefUpdated),
          checkPipelineAttributes(pipelineDefUpdated),
        ),
      },
      {
        ResourceName:             tfPipelineResourceReference,
        ImportState:              true,
        ImportStateVerify:        true,
        // During the import the provider has no access to the configuration thus the pipeline definition is entirely imported from Spinnaker
        ImportStateVerifyIgnore:  []string{"pipeline.definition"},
        ImportStateCheck:         importCheckPipeline(),
      },
    },
  })
}

func TestAccPipelineResource_fail(t *testing.T) {
  name := acctest.RandomWithPrefix("tf-acc-test")

  resource.ParallelTest(t, resource.TestCase{
    ProtoV6ProviderFactories: protoV6ProviderFactories,
    Steps: []resource.TestStep{
      {
        Config: testAccPipelineResourceConfig_fail(name),
        ExpectError: regexp.MustCompile("The pipeline definition is not valid: missing field[[:space:]]*pipeline.definition.stages"),
      },
    },
  })
}

func TestAccPipelineResource_complex(t *testing.T) {
  name := acctest.RandomWithPrefix("tf-acc-test")
  pipelineDef := generatePipelineConfig_complex(name, name, true)
  nameUpdated := acctest.RandomWithPrefix("tf-acc-test")
  pipelineDefUpdated := generatePipelineConfig_complex(name, nameUpdated, true)

  resource.ParallelTest(t, resource.TestCase{
    ProtoV6ProviderFactories: protoV6ProviderFactories,
    Steps: []resource.TestStep{
      {
        Config: testAccPipelineResourceConfig_complex(name, name),
        Check: resource.ComposeAggregateTestCheckFunc(
          checkPipelineExists(),
					resource.TestCheckResourceAttr(tfPipelineResourceReference, "pipeline.definition", pipelineDef),
          checkPipelineAttributes(pipelineDef),
        ),
      },
      {
        Config: testAccPipelineResourceConfig_complex(name, nameUpdated),
        Check:  resource.ComposeAggregateTestCheckFunc(
          checkPipelineExists(),
					resource.TestCheckResourceAttr(tfPipelineResourceReference, "pipeline.definition", pipelineDefUpdated),
          checkPipelineAttributes(pipelineDefUpdated),
        ),
      },
      {
        ResourceName:             tfPipelineResourceReference,
        ImportState:              true,
        ImportStateVerify:        true,
        ImportStateVerifyIgnore:  []string{"pipeline.definition"},
        ImportStateCheck:         importCheckPipeline(),
      },
    },
  })
}

func generatePipelineConfig_basic(applicationName, pipelineName string, expanded bool) string {
  var application string
  if expanded {
	  application = applicationName
  } else {
	  application = fmt.Sprintf("${resource.%s.name}", tfApplicationResourceReference)
  }
  pipelineConfig := map[string]interface{}{
    "application": application,
    "name": pipelineName,
    "stages": make([]interface{}, 0),
    "triggers": make([]interface{}, 0),
  }
  pipelineConfigBytes, _ := json.Marshal(pipelineConfig)
  return string(pipelineConfigBytes)
}

func testAccPipelineResourceConfig_basic(applicationName, pipelineName string) string {
  pipelineConfig := generatePipelineConfig_basic(applicationName, pipelineName, false)

  return fmt.Sprintf(`
%s

resource spinnaker_pipeline "test" {
  pipeline = {
    definition = jsonencode(%s)
  }
  provisioner "local-exec" {
    command = "sleep 5s"
  }
}
  `, testAccApplicationResourceConfig_basic(applicationName, nil), pipelineConfig)
}

func testAccPipelineResourceConfig_fail(name string) string {
  return fmt.Sprintf(`
%s

resource spinnaker_pipeline "test" {
  pipeline = {
    definition = jsonencode({
      application = resource.%s.name
      name = %q
      triggers = []
    })
  }
}
  `, testAccApplicationResourceConfig_basic(name, nil), tfApplicationResourceReference, name)
}

func generatePipelineConfig_complex(applicationName, pipelineName string, expanded bool) string {
  var application string
  if expanded {
	  application = applicationName
  } else {
	  application = fmt.Sprintf("${resource.%s.name}", tfApplicationResourceReference)
  }
  pipelineConfig := map[string]interface{}{
	  "application": application,
    "keepWaitingPipelines": false,
    "limitConcurrent": true,
    "name": pipelineName,
    "spelEvaluator": "v4",
    "stages": []map[string]interface{}{
      {
        "name": "Wait",
        "refId": 1,
        "requisiteStageRefIds": make([]string, 0),
        "type": "wait",
        "waitTime": 30,
      },
      {
        "failPipeline": true,
        "judgmentInputs": make([]interface{}, 0),
        "name": "Manual Judgment",
        "notifications": make([]interface{}, 0),
        "refId": 2,
        "requisiteStageRefIds": []string{"1"},
        "type": "manualJudgment",
      },
    },
    "triggers": []map[string]interface{}{
			{
			 "enabled": true,
			 "source": "example",
			 "type": "webhook",
			},
    },
  }
  pipelineConfigBytes, _ := json.Marshal(pipelineConfig)
  return string(pipelineConfigBytes)
}

func testAccPipelineResourceConfig_complex(applicationName, pipelineName string) string {
  pipelineConfig := generatePipelineConfig_complex(applicationName, pipelineName, false)

  return fmt.Sprintf(`
%s

resource spinnaker_pipeline "test" {
  pipeline = {
    definition = jsonencode(%s)
  }
  provisioner "local-exec" {
    command = "sleep 5s"
  }
}
  `, testAccApplicationResourceConfig_basic(applicationName, nil), pipelineConfig)
}

func checkPipelineExists() resource.TestCheckFunc {
  return func(s *terraform.State) error {
    rs, ok := s.RootModule().Resources[tfPipelineResourceReference]
    if !ok {
      return fmt.Errorf("Pipeline not found: %s", tfPipelineResourceReference)
    }
    if rs.Primary.ID == "" {
      return fmt.Errorf("No Application ID is set")
    }

    client, err := newSpinClient("", "", false)
    if err != nil {
      return err
    }
    api := api.Spinnaker{Client: client}

    pipelineRes := make(map[string]interface{})
    diag := readPipelineFromString(rs.Primary.Attributes["pipeline.result"], &pipelineRes)
    if diag.HasError() {
      return fmt.Errorf("%v", diag)
    }

    _, err = api.GetPipeline(context.Background(), pipelineRes["application"].(string), pipelineRes["name"].(string))
    return err
  }
}

func checkPipelineAttributes(pipelineDef_str string) resource.TestCheckFunc {
  return func(s *terraform.State) error {
    var pipelineDef, pipelineRes map[string]interface{}
    var diag diag.Diagnostics
    diag.Append(readPipelineFromString(pipelineDef_str, &pipelineDef)...)
    diag.Append(readPipelineFromString(s.RootModule().Resources[tfPipelineResourceReference].Primary.Attributes["pipeline.result"], &pipelineRes)...)
    if diag.HasError() {
			return fmt.Errorf(diag.Errors()[0].Summary())
    }
		return comparePipelineValues(reflect.ValueOf(pipelineDef), reflect.ValueOf(pipelineRes), "pipeline.definition", "pipeline.result")
	}
}

// Checks that v1 is equal or is a subset of v2
func comparePipelineValues(v1, v2 reflect.Value, path1, path2 string) (err error) {
	if v1.Kind() != v2.Kind() {
		return fmt.Errorf("Pipeline configuration type mismatch for the field %s (value: %s - kind: %s) != %s (value: %s - kind: %s)", path1, v1, v1.Kind(), path2, v2, v2.Kind())
	}
  switch v1.Kind() {
  case reflect.Slice:
    for i := 0; i < v1.Len(); i++ {
      fieldPath1 := fmt.Sprintf("%s[%d]", path1, i)
      fieldPath2 := fmt.Sprintf("%s[%d]", path2, i)
      if err = comparePipelineValues(reflect.ValueOf(v1.Index(i).Interface()), reflect.ValueOf(v2.Index(i).Interface()), fieldPath1, fieldPath2); err != nil {
        break
      }
    }
  case reflect.Map:
		for _, key := range v1.MapKeys() {
      fieldPath1 := fmt.Sprintf("%s.%s", path1, key)
      fieldPath2 := fmt.Sprintf("%s.%s", path2, key)
      if err = comparePipelineValues(reflect.ValueOf(v1.MapIndex(key).Interface()), reflect.ValueOf(v2.MapIndex(key).Interface()), fieldPath1, fieldPath2); err != nil {
        break
      }
		}
  case reflect.Invalid:
    err = fmt.Errorf("Invalid field value %s (value: %s)", path1, v1)
  default:
		if v1.Interface() != v2.Interface() {
			err = fmt.Errorf("Pipeline configuration mismatch for the field %s (value: %s) != %s (value: %s)", path1, v1.Interface(), path2, v2.Interface())
		}
  }
  return err
}

func importCheckPipeline() resource.ImportStateCheckFunc {
  return func(stateList []*terraform.InstanceState) error {
    if len(stateList) != 1 {
      return fmt.Errorf("importCheckPipeline: expecting 1 state, got: %d", len(stateList))
    }
    state := stateList[0]

    var diag diag.Diagnostics
    for _, attr := range []string{"pipeline.definition", "pipeline.result"} {
      str, ok := state.Attributes[attr]
      if !ok {
        return fmt.Errorf("importCheckPipeline: missing attribute %s", attr)
      }
      var pipeline map[string]interface{}
      diag.Append(readPipelineFromString(str, &pipeline)...)
      if diag.HasError() {
        return fmt.Errorf(diag.Errors()[0].Summary())
      }
      if len(pipeline) == 0 {
        return fmt.Errorf("importCheckPipeline: empty attribute %s", attr)
      }
    }

    return nil
  }
}
