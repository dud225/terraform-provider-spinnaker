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

const tfApplicationResourceReference = "spinnaker_application.test"

func TestAccApplicationResource_basic_ok(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
	nameUpdated := acctest.RandomWithPrefix("tf-acc-test")
  optionEmailUpdated := applicationOptions{email: "test-updated@example.com"}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationResourceConfig_basic(name, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkApplicationExists(),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "name", name),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "email", "test@example.com"),
				),
			},
			{
				Config: testAccApplicationResourceConfig_basic(name, &optionEmailUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkApplicationExists(),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "name", name),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "email", "test-updated@example.com"),
				),
			},
			{
				Config: testAccApplicationResourceConfig_basic(nameUpdated, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkApplicationExists(),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "name", nameUpdated),
				),
			},
			{
				ResourceName:      tfApplicationResourceReference,
				ImportState:       true,
				ImportStateVerify: true,
        ImportStateCheck:  importCheckApplication([]string{}),
			},
		},
	})
}

func TestAccApplicationResource_basic_fail(t *testing.T) {
	name := acctest.RandomWithPrefix("TF-ACC-TEST")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationResourceConfig_basic(name, nil),
				ExpectError: regexp.MustCompile("Attribute name shall be set in lowercase"),
			},
		},
	})
}

func TestAccApplicationResource_cloudproviders_ok(t *testing.T) {
  // Name has to match provider constraints
  // See: https://github.com/spinnaker/orca/blob/master/orca-applications/src/main/groovy/com/netflix/spinnaker/orca/applications/utils/ApplicationNameValidator.groovy
	name := "tfacctest"
  optionCloudProvidersUpdated := applicationOptions{cloudProviders: []string{"kubernetes", "openstack"}}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationResourceConfig_cloudproviders_ok(name, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkApplicationExists(),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "name", name),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "cloud_providers.0", "kubernetes"),
				),
			},
			{
				Config: testAccApplicationResourceConfig_cloudproviders_ok(name, &optionCloudProvidersUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkApplicationExists(),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "name", name),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "cloud_providers.0", "kubernetes"),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "cloud_providers.1", "openstack"),
				),
			},
			{
				ResourceName:      tfApplicationResourceReference,
				ImportState:       true,
				ImportStateVerify: true,
        ImportStateCheck:  importCheckApplication([]string{"cloud_providers.0", "cloud_providers.1"}),
			},
		},
	})
}

func TestAccApplicationResource_cloudproviders_fail(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationResourceConfig_cloudproviders_fail(name),
				ExpectError: regexp.MustCompile("Attribute cloud_providers.*Value must be one of:"),
			},
		},
	})
}

func TestAccApplicationResource_permissions_ok(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationResourceConfig_permissions_ok(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkApplicationExists(),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "name", name),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "permissions.role1.0", "READ"),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "permissions.role2.0", "EXECUTE"),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "permissions.role2.1", "READ"),
					resource.TestCheckResourceAttr(tfApplicationResourceReference, "permissions.role2.2", "WRITE"),
				),
			},
			{
				ResourceName:      tfApplicationResourceReference,
				ImportState:       true,
				ImportStateVerify: true,
        ImportStateCheck:  importCheckApplication([]string{"permissions.role1.0", "permissions.role2.0", "permissions.role2.1", "permissions.role2.2"}),
			},
		},
	})
}

func TestAccApplicationResource_permissions_fail(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationResourceConfig_permissions_fail(name),
				ExpectError: regexp.MustCompile("Attribute permission.*Value must be one of:"),
			},
		},
	})
}

type applicationOptions struct {
  email string
  cloudProviders []string
  permissions map[string][]string
}

func testAccApplicationResourceConfig_basic(name string, options *applicationOptions) string {
  email := "test@example.com"
  if options != nil && options.email != "" {
    email = options.email
  }
  return fmt.Sprintf(`
resource spinnaker_application "test" {
  name = %q
	email = %q
}
	`, name, email)
}

func testAccApplicationResourceConfig_cloudproviders_ok(name string, options *applicationOptions) string {
  cloudProviders := []string{"kubernetes"}
  if options != nil && options.cloudProviders != nil {
    cloudProviders = options.cloudProviders
  }
  cloudProviders_json, _ := json.Marshal(cloudProviders)
  return fmt.Sprintf(`
resource spinnaker_application "test" {
  name = %q
	email = "test@example.com"
	cloud_providers = %s
}
	`, name, cloudProviders_json)
}

func testAccApplicationResourceConfig_cloudproviders_fail(name string) string {
  return fmt.Sprintf(`
resource spinnaker_application "test" {
  name = %q
	email = "test@example.com"
	cloud_providers = [ "gogol_compute_engine" ]
}
	`, name)
}

func testAccApplicationResourceConfig_permissions_ok(name string) string {
  return fmt.Sprintf(`
resource spinnaker_application "test" {
  name = %q
	email = "test@example.com"
	permissions = {
		"role1" = [ "READ" ],
		"role2" = [ "READ", "WRITE", "EXECUTE" ]
	}
}
	`, name)
}

func testAccApplicationResourceConfig_permissions_fail(name string) string {
  return fmt.Sprintf(`
resource spinnaker_application "test" {
  name = %q
	email = "test@example.com"
	permissions = {
		"role1" = [ "WRONG_PERM" ]
	}
}
	`, name)
}

func checkApplicationExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
    rs, ok := s.RootModule().Resources[tfApplicationResourceReference]
    if !ok {
      return fmt.Errorf("Application not found: %s", tfApplicationResourceReference)
    }
    if rs.Primary.ID == "" {
      return fmt.Errorf("No Application ID is set")
    }

		client, err := newSpinClient("", "", false)
		if err != nil {
			return err
		}
		_, err = api.Spinnaker{Client: client}.GetApplication(context.Background(), rs.Primary.ID)
		return err
	}
}

func importCheckApplication(attrs []string) resource.ImportStateCheckFunc {
  return func(stateList []*terraform.InstanceState) error {
    if len(stateList) != 1 {
      return fmt.Errorf("importCheckApplication: expecting 1 state, got: %d", len(stateList))
    }
    state := stateList[0]

    for _, attr := range attrs {
      _, ok := state.Attributes[attr]
      if !ok {
        return fmt.Errorf("importCheckApplication: missing attribute %s; attributes present: %v", attr, state.Attributes)
      }
    }

    return nil
  }
}
