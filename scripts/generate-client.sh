#!/bin/bash
# Generates the Go API client from backend-core's swagger.json.
#
# 1. redocly filters the spec down to the automation API surface
#    (tags listed in redocly.yaml), same mechanism as snapshot-save.
# 2. openapi-generator produces the Go client into internal/wbclient.
#
# Requires: redocly CLI, docker, go. backend-core must be checked out as a
# sibling directory (../backend-core), matching redocly.yaml's root.
set -euo pipefail

cd "$(dirname "$0")/.."

GENERATOR_IMAGE="openapitools/openapi-generator-cli:v7.7.0"
CLIENT_DIR="internal/wbclient"
FILTERED_SPEC="docs/swagger-filtered.json"

if ! command -v redocly &> /dev/null; then
	echo "redocly could not be found"
	echo "Visit 'https://redocly.com/docs/cli/installation' to install redocly"
	exit 1
fi

if [ ! -f ../backend-core/docs/swagger.json ]; then
	echo "../backend-core/docs/swagger.json not found; check out backend-core as a sibling directory"
	exit 1
fi

echo "Generating filtered spec -> $FILTERED_SPEC"
mkdir -p docs
redocly bundle filter -o "$FILTERED_SPEC" --ext json --remove-unused-components

echo "Generating Go client -> $CLIENT_DIR"
rm -rf "$CLIENT_DIR"
docker run --rm -v "${PWD}:/local" "$GENERATOR_IMAGE" generate \
	-i "/local/$FILTERED_SPEC" \
	-g go \
	-o "/local/$CLIENT_DIR" \
	--additional-properties=packageName=wbclient \
	--additional-properties=generateInterfaces=true \
	--additional-properties=withGoMod=false \
	--additional-properties=disallowAdditionalPropertiesIfNotPresent=false \
	--skip-validate-spec

# Drop generated scaffolding we don't want in the repo.
rm -rf "$CLIENT_DIR/test" "$CLIENT_DIR/docs" "$CLIENT_DIR/api"
rm -f "$CLIENT_DIR/git_push.sh" "$CLIENT_DIR/.travis.yml" "$CLIENT_DIR/.gitignore"

# Docker writes as root on Linux runners; make sure the tree is ours.
if [ "$(uname)" = "Linux" ] && command -v sudo &> /dev/null; then
	sudo chown -R "$(id -u):$(id -g)" "$CLIENT_DIR" "$FILTERED_SPEC" || true
fi

gofmt -w "$CLIENT_DIR"
go mod tidy
go build ./...

echo "Done!"
