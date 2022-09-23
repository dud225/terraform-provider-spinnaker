package provider

import (
  "fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

const tfPipelineDataSourceReference = "data.spinnaker_pipeline.test"

func TestAccPipelineDataSource_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
  pipelineDef := generatePipelineConfig_basic(name, name, true)
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPipelineDataSourceConfig_basic(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfPipelineResourceReference, "pipeline.definition", pipelineDef),
          checkPipelineAttributes(pipelineDef),
				),
			},
    },
  })
}


func TestAccPipelineDataSource_complex(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
  pipelineDef := generatePipelineConfig_complex(name, name, true)
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPipelineDataSourceConfig_complex(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfPipelineResourceReference, "pipeline.definition", pipelineDef),
          checkPipelineAttributes(pipelineDef),
				),
			},
    },
  })
}

func testAccPipelineDataSourceConfig_basic(name string) string {
  return fmt.Sprintf(`
%s

data spinnaker_pipeline "test" {
  name = resource.%s.name
}
  `, testAccPipelineResourceConfig_basic(name, name), tfPipelineResourceReference)
}

func testAccPipelineDataSourceConfig_complex(name string) string {
  return fmt.Sprintf(`
%s

data spinnaker_pipeline "test" {
  name = resource.%s.name
}
  `, testAccPipelineResourceConfig_complex(name, name), tfPipelineResourceReference)
}
