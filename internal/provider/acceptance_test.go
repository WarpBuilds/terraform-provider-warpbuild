package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Acceptance tests run against a real WarpBuild environment and are gated on
// TF_ACC=1. Required environment:
//
//	WARPBUILD_API_KEY       API key for the test org
//	WARPBUILD_API_ENDPOINT  API endpoint (e.g. the preprod API)
//
// Optional overrides (defaults target the preprod test org):
//
//	WARPBUILD_ACC_STACK_ALIAS    EC2 stack alias to deploy into
//	WARPBUILD_ACC_AMI_ID         AMI for image create
//	WARPBUILD_ACC_AMI_ID_UPDATE  AMI (same os/arch) for image update
//	WARPBUILD_ACC_NAME_PREFIX    org-enforced runner name prefix
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"warpbuild": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, v := range []string{"WARPBUILD_API_KEY", "WARPBUILD_API_ENDPOINT"} {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}

func accEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func accStackAlias() string  { return accEnv("WARPBUILD_ACC_STACK_ALIAS", "test-aws-stack") }
func accAmiID() string       { return accEnv("WARPBUILD_ACC_AMI_ID", "ami-061b2077fdbe1f8ae") }
func accAmiIDUpdate() string { return accEnv("WARPBUILD_ACC_AMI_ID_UPDATE", "ami-079b8fd4a9f09ce6d") }
func accNamePrefix() string  { return accEnv("WARPBUILD_ACC_NAME_PREFIX", "warpdev-custom-") }

func TestAccRunnerImage_lifecycle(t *testing.T) {
	alias := acctest.RandomWithPrefix("tf-acc-img")

	config := func(amiID string) string {
		return fmt.Sprintf(`
provider "warpbuild" {}

data "warpbuild_stack" "ec2" {
  alias = %[1]q
}

resource "warpbuild_runner_image" "test" {
  alias    = %[2]q
  stack_id = data.warpbuild_stack.ec2.id
  ami_id   = %[3]q
}

data "warpbuild_runner_image" "by_alias" {
  alias      = warpbuild_runner_image.test.alias
  depends_on = [warpbuild_runner_image.test]
}
`, accStackAlias(), alias, amiID)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(accAmiID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpbuild_runner_image.test", "alias", alias),
					resource.TestCheckResourceAttr("warpbuild_runner_image.test", "ami_id", accAmiID()),
					resource.TestCheckResourceAttrSet("warpbuild_runner_image.test", "id"),
					resource.TestCheckResourceAttrSet("warpbuild_runner_image.test", "os"),
					resource.TestCheckResourceAttrSet("warpbuild_runner_image.test", "arch"),
					resource.TestCheckResourceAttr("warpbuild_runner_image.test", "status", "available"),
					resource.TestCheckResourceAttrPair(
						"data.warpbuild_runner_image.by_alias", "id",
						"warpbuild_runner_image.test", "id"),
				),
			},
			{
				// In-place AMI update creates a new image version.
				Config: config(accAmiIDUpdate()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpbuild_runner_image.test", "ami_id", accAmiIDUpdate()),
				),
			},
			{
				ResourceName:      "warpbuild_runner_image.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccRunner_lifecycle(t *testing.T) {
	name := accNamePrefix() + acctest.RandomWithPrefix("tf-acc")

	config := func(poolSize int, extraLabel string) string {
		labels := ""
		if extraLabel != "" {
			labels = fmt.Sprintf("labels = [%q, %q]", name, extraLabel)
		}
		return fmt.Sprintf(`
provider "warpbuild" {}

data "warpbuild_stack" "ec2" {
  alias = %[1]q
}

resource "warpbuild_runner_image" "test" {
  alias    = %[2]q
  stack_id = data.warpbuild_stack.ec2.id
  ami_id   = %[3]q
}

resource "warpbuild_runner" "test" {
  name        = %[4]q
  provider_id = data.warpbuild_stack.ec2.id
  pool_size   = %[5]d
  %[6]s

  configuration = {
    image = warpbuild_runner_image.test.id
    byoc_sku = {
      arch           = "x64"
      instance_types = ["m7a.large"]
      is_public      = true
    }
    storage = {
      tier       = "custom"
      size       = 150
      iops       = 3000
      throughput = 125
    }
  }
}
`, accStackAlias(), name+"-img", accAmiID(), name, poolSize, labels)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(0, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpbuild_runner.test", "name", name),
					resource.TestCheckResourceAttr("warpbuild_runner.test", "pool_size", "0"),
					resource.TestCheckResourceAttr("warpbuild_runner.test", "configuration.capacity_type", "ondemand"),
					resource.TestCheckTypeSetElemAttr("warpbuild_runner.test", "labels.*", name),
					resource.TestCheckResourceAttrSet("warpbuild_runner.test", "id"),
				),
			},
			{
				// pool_size flows through PATCH /runners; labels update in place.
				Config: config(1, "tf-acc-extra"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpbuild_runner.test", "pool_size", "1"),
					resource.TestCheckTypeSetElemAttr("warpbuild_runner.test", "labels.*", "tf-acc-extra"),
				),
			},
			{
				// Import recovers pool_size via the pools API.
				ResourceName:      "warpbuild_runner.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Scale the warm pool back down before destroy.
				Config: config(0, ""),
				Check:  resource.TestCheckResourceAttr("warpbuild_runner.test", "pool_size", "0"),
			},
		},
	})
}

func TestAccRunner_spotPoolRejected(t *testing.T) {
	config := fmt.Sprintf(`
provider "warpbuild" {}

resource "warpbuild_runner" "test" {
  name        = %[1]q
  provider_id = "irrelevant"
  pool_size   = 1

  configuration = {
    capacity_type = "spot"
    image         = "irrelevant"
    byoc_sku = {
      arch           = "x64"
      instance_types = ["m7a.large"]
    }
  }
}
`, accNamePrefix()+"tf-acc-spot")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile("Warm pools are not supported for spot runners"),
			},
		},
	})
}
