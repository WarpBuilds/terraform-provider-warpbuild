# terraform-provider-warpbuild

Terraform provider for [WarpBuild](https://www.warpbuild.com)'s automation API:
stacks, BYOC (AWS AMI) runner images, and custom runner sets. See the
[automation API docs](https://www.warpbuild.com/docs/ci/api-keys/automation).

## API client

`internal/wbclient` is generated — do not edit by hand.

The client is produced from backend-core's `docs/swagger.json`, filtered down
to the automation API tags via redocly (`redocly.yaml`, the same mechanism as
snapshot-save), then run through openapi-generator (Go, pinned in
`scripts/generate-client.sh`).

To regenerate (backend-core checked out as a sibling directory):

```sh
./scripts/generate-client.sh
```

Regeneration is local-only; rerun the script and commit the result when the
backend API changes.
