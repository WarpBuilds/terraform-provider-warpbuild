data "warpbuild_stack" "ec2" {
  alias = "my-ec2-stack"
}

resource "warpbuild_runner" "custom" {
  name        = "my-custom-runner"
  provider_id = data.warpbuild_stack.ec2.id

  # Warm pool instances kept ready for jobs; ondemand runners only.
  pool_size = 1

  configuration = {
    image = warpbuild_runner_image.custom.id

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
