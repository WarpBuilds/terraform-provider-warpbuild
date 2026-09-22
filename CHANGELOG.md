# Changelog

## 0.3.0-beta (2026-09-22)

### Fixed

- `warpbuild_runner` - setting `labels` no longer fails the apply with
  "Provider produced inconsistent result after apply".
  Support `labels = ["dependabot", "code-scanning"]` - the
  Allow Dependabot and Allow CodeQL checkboxes in the UI

### Changed

- Updated example runner to carry the `warp-custom-` prefix the API requires.

### Notes

- The API lowercases labels and appends the runner name, so the set stored in
  state is what you configured, not what the API returns. A runner answers to
  its own name whether or not it is listed in `labels`.
- Removing the `labels` attribute from a resource that had it is a no-op, not a
  reset - the previous value is retained. Use `labels = []` to clear it.
- An imported runner carries the runner name as a label, so the first plan
  after `terraform import` shows one reconciling change to `labels`.

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
