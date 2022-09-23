package provider

import (
  "fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

const tfProjectDataSourceReference = "data.spinnaker_project.test"

func TestAccProjectDataSource_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
  defaultConfig, err := projectDefaultConfig()
  if err != nil {
    t.Fatalf(err.Error())
  }

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectDataSourceConfig_basic(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfProjectDataSourceReference, "name", name),
					resource.TestCheckResourceAttr(tfProjectDataSourceReference, "email", "test@example.com"),
					resource.TestCheckResourceAttr(tfProjectDataSourceReference, "config", defaultConfig),
				),
			},
    },
  })
}


func TestAccProjectDataSource_config(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectDataSourceConfig_config(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfProjectDataSourceReference, "name", name),
					resource.TestCheckResourceAttr(tfProjectResourceReference, "config", generateProjectConfig(name, true)),
				),
			},
    },
  })
}

func testAccProjectDataSourceConfig_basic(name string) string {
  return fmt.Sprintf(`
%s

data spinnaker_project "test" {
  name = resource.%s.name
}
  `, testAccProjectResourceConfig_basic(name), tfProjectResourceReference)
}

func testAccProjectDataSourceConfig_config(name string) string {
  return fmt.Sprintf(`
%s

data spinnaker_project "test" {
  name = resource.%s.name
}
  `, testAccProjectResourceConfig_config_ok(name), tfProjectResourceReference)
}
