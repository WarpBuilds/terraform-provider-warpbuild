# Changelog

## 0.1.0 (2026-07-17)

Initial release.

### Resources

- `warpbuild_runner_image` — BYOC AWS AMI runner images. OS, architecture and
  root device are derived from the AMI; updating `ami_id` creates a new image
  version in place. Import by ID.
- `warpbuild_runner` — custom BYOC runner sets: instance selection
  (`byoc_sku`), storage, labels, and warm pool size. Import by ID (recovers
  `pool_size`).

### Data sources

- `warpbuild_stack` — look up a stack by alias/region (`kind` defaults to
  `ec2`).
- `warpbuild_runner_image` — look up an existing runner image by alias.

### Notes

- Warm pools are validated against spot runners (unsupported server-side).
- Only the `byoc_ami` image flow is supported; container and
  WarpBuild-managed image flows are intentionally not expressible.
