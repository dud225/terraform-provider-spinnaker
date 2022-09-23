package provider

import (
	"context"
  "encoding/json"
	"fmt"
	"regexp"
	"testing"

  "github.com/dud225/terraform-provider-spinnaker/internal/api"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const tfProjectResourceReference = "spinnaker_project.test"

func TestAccProjectResource_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
	nameUpdated := acctest.RandomWithPrefix("tf-acc-test")
  defaultConfig, err := projectDefaultConfig()
  if err != nil {
    t.Fatalf(err.Error())
  }

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectResourceConfig_basic(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkProjectExists(),
					resource.TestCheckResourceAttr(tfProjectResourceReference, "name", name),
					resource.TestCheckResourceAttr(tfProjectResourceReference, "email", "test@example.com"),
					resource.TestCheckResourceAttr(tfProjectResourceReference, "config", defaultConfig),
				),
			},
			{
				Config: testAccProjectResourceConfig_basic(nameUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkProjectExists(),
					resource.TestCheckResourceAttr(tfProjectResourceReference, "name", nameUpdated),
				),
			},
			{
				ResourceName:      tfProjectResourceReference,
				ImportState:       true,
				ImportStateVerify: true,
        ImportStateCheck:  importCheckApplication([]string{"email", "config"}),
			},
		},
	})
}

func TestAccProjectResource_config_ok(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectResourceConfig_config_ok(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkProjectExists(),
					resource.TestCheckResourceAttr(tfProjectResourceReference, "name", name),
					resource.TestCheckResourceAttr(tfProjectResourceReference, "config", generateProjectConfig(name, true)),
				),
			},
			{
				ResourceName:      tfProjectResourceReference,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccProjectResource_config_fail(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectResourceConfig_config_fail(name),
				ExpectError: regexp.MustCompile("The project configuration is not valid: missing field config.applications"),
			},
		},
	})
}

func testAccProjectResourceConfig_basic(name string) string {
  return fmt.Sprintf(`
resource spinnaker_project "test" {
  name = %q
	email = "test@example.com"
}
	`, name)
}

func generateProjectConfig(applicationName string, expanded bool) string {
  var application string
  if expanded {
	  application = applicationName
  } else {
	  application = fmt.Sprintf("${resource.%s.name}", tfApplicationResourceReference)
  }
  projectConfig := map[string]interface{}{
	  "applications": []string{application},
    "clusters": make([]map[string]interface{}, 0),
    "pipelineConfigs": make([]map[string]interface{}, 0),
  }
  projectConfigBytes, _ := json.Marshal(projectConfig)
  return string(projectConfigBytes)
}

func testAccProjectResourceConfig_config_ok(name string) string {
  projectConfig := generateProjectConfig(name, false)

  return fmt.Sprintf(`
%s

resource spinnaker_project "test" {
  name = %q
  email = "test@example.com"
  config = jsonencode(%s)
}
	`, testAccApplicationResourceConfig_basic(name, nil), name, projectConfig)
}

func testAccProjectResourceConfig_config_fail(name string) string {
  return fmt.Sprintf(`
resource spinnaker_project "test" {
  name = %q
  email = "test@example.com"
  config = jsonencode({
    apps = []
    clusters = []
    pipelineConfigs = []
  })
}
	`, name)
}

func checkProjectExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
    rs, ok := s.RootModule().Resources[tfProjectResourceReference]
    if !ok {
      return fmt.Errorf("Project not found: %s", tfProjectResourceReference)
    }
    if rs.Primary.ID == "" {
      return fmt.Errorf("No Application ID is set")
    }

		client, err := newSpinClient("", "", false)
		if err != nil {
			return err
		}
		_, err = api.Spinnaker{Client: client}.GetProject(context.Background(), rs.Primary.ID)
		return err
	}
}

func importCheckProject(attrs []string) resource.ImportStateCheckFunc {
  return func(stateList []*terraform.InstanceState) error {
    if len(stateList) != 1 {
      return fmt.Errorf("importCheckProject: expecting 1 state, got: %d", len(stateList))
    }
    state := stateList[0]

    for _, attr := range attrs {
      value, ok := state.Attributes[attr]
      if !ok {
        return fmt.Errorf("importCheckProject: missing attribute %s; attributes present: %v", attr, state.Attributes)
      }
      if len(value) == 0 {
        return fmt.Errorf("importCheckProject: empty attribute %s", attr)
      }
    }

    return nil
  }
}
