terraform {
  required_providers {
    warpbuild = {
      source = "warpbuilds/warpbuild"
    }
  }
}

# API key via WARPBUILD_API_KEY env var, or set api_key here.
provider "warpbuild" {}

# Look up the EC2 stack to deploy into.
data "warpbuild_stack" "ec2" {
  alias = "my-ec2-stack"
}

# A BYOC AMI runner image.
resource "warpbuild_runner_image" "custom" {
  alias    = "my-custom-image"
  stack_id = data.warpbuild_stack.ec2.id
  ami_id   = "ami-0123456789abcdef0"

  purge_image_versions_offset = 2
}

# A custom runner set using the image.
resource "warpbuild_runner" "custom" {
  name        = "my-custom-runner"
  provider_id = data.warpbuild_stack.ec2.id
  pool_size   = 1

  configuration = {
    capacity_type = "ondemand"
    image         = warpbuild_runner_image.custom.id

    byoc_sku = {
      arch           = "x64"
      instance_types = ["m5.xlarge", "m5.2xlarge"]
      role_arn       = "arn:aws:iam::123456789012:role/WarpBuildRunnerRole"
    }

    storage = {
      tier = "custom"
      size = 256
    }
  }
}
