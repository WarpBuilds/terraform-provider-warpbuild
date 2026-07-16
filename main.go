package main

import (
	"context"
	"flag"
	"log"

	"github.com/WarpBuilds/terraform-provider-warpbuild/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// version is set by goreleaser at release time.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with debugger support")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/warpbuilds/warpbuild",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
