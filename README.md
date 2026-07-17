# terraform-provider-warpbuild

Terraform provider for [WarpBuild](https://www.warpbuild.com)'s automation API:
stacks, BYOC (AWS AMI) runner images, and custom runner sets. See the
[automation API docs](https://www.warpbuild.com/docs/ci/api-keys/automation).

## Usage

```hcl
terraform {
  required_providers {
    warpbuild = {
      source  = "warpbuilds/warpbuild"
      version = "~> 0.1"
    }
  }
}

# API key with the `ci` scope; may also be set via WARPBUILD_API_KEY.
provider "warpbuild" {}

data "warpbuild_stack" "ec2" {
  alias = "my-ec2-stack"
}

resource "warpbuild_runner_image" "custom" {
  alias    = "my-custom-image"
  stack_id = data.warpbuild_stack.ec2.id
  ami_id   = "ami-0123456789abcdef0"
}

resource "warpbuild_runner" "custom" {
  name        = "my-custom-runner"
  provider_id = data.warpbuild_stack.ec2.id
  pool_size   = 1

  configuration = {
    image = warpbuild_runner_image.custom.id
    byoc_sku = {
      arch           = "x64"
      instance_types = ["m5.xlarge"]
      role_arn       = "arn:aws:iam::123456789012:role/WarpBuildRunnerRole"
    }
  }
}
```

Use the runner's labels in your workflows (e.g. `runs-on: my-custom-runner`).
Full documentation for every resource and data source lives in
[`docs/`](docs/) and on the Terraform Registry.

## Development

### API client

`internal/wbclient` is generated — do not edit by hand.

The client is produced from backend-core's `docs/swagger.json`, filtered down
to the automation API operations via redocly (`redocly.yaml`), then run
through openapi-generator (Go, pinned in `scripts/generate-client.sh`).

To regenerate (backend-core checked out as a sibling directory):

```sh
./scripts/generate-client.sh
```

Spec changes in backend-core raise sync PRs here automatically via
backend-core's code-gen workflow.

### Testing

Acceptance tests run against a real WarpBuild environment:

```sh
export WARPBUILD_API_KEY=...           # test org API key
export WARPBUILD_API_ENDPOINT=...     # test environment API endpoint
TF_ACC=1 go test ./internal/provider/ -v -timeout 30m
```

### Releasing

Push a `v*` tag; GoReleaser builds, signs and publishes the registry
artifacts (see `.github/workflows/release.yaml`).
