package provider

import (
  "fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

const tfApplicationDataSourceReference = "data.spinnaker_application.test"

func TestAccApplicationDataSource_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationDataSourceConfig_basic(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "name", name),
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "email", "test@example.com"),
				),
			},
    },
  })
}


func TestAccApplicationDataSource_cloudproviders(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationDataSourceConfig_cloudproviders(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "name", name),
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "cloud_providers.0", "kubernetes"),
				),
			},
    },
  })
}

func TestAccApplicationDataSource_permissions(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationDataSourceConfig_permissions(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "name", name),
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "permissions.role1.0", "READ"),
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "permissions.role2.0", "EXECUTE"),
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "permissions.role2.1", "READ"),
					resource.TestCheckResourceAttr(tfApplicationDataSourceReference, "permissions.role2.2", "WRITE"),
				),
			},
    },
  })
}

func testAccApplicationDataSourceConfig_basic(name string) string {
  return fmt.Sprintf(`
%s

data spinnaker_application "test" {
  name = resource.%s.name
}
  `, testAccApplicationResourceConfig_basic(name, nil), tfApplicationResourceReference)
}

func testAccApplicationDataSourceConfig_cloudproviders(name string) string {
  return fmt.Sprintf(`
%s

data spinnaker_application "test" {
  name = resource.%s.name
}
  `, testAccApplicationResourceConfig_cloudproviders_ok(name, nil), tfApplicationResourceReference)
}

func testAccApplicationDataSourceConfig_permissions(name string) string {
  return fmt.Sprintf(`
%s

data spinnaker_application "test" {
  name = resource.%s.name
}
  `, testAccApplicationResourceConfig_permissions_ok(name), tfApplicationResourceReference)
}
