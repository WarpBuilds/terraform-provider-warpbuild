data "warpbuild_stack" "ec2" {
  alias = "my-ec2-stack"
}

resource "warpbuild_runner_image" "custom" {
  alias    = "my-custom-image"
  stack_id = data.warpbuild_stack.ec2.id
  ami_id   = "ami-0123456789abcdef0"
}
