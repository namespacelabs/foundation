// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"os"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/sdk/buf/image"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func main() {
	targetPlatform := os.Getenv("TARGET_PLATFORM")
	if targetPlatform == "" {
		log.Fatal("TARGET_PLATFORM is missing")
	}

	platform, err := devhost.ParsePlatform(targetPlatform)
	if err != nil {
		log.Fatal(err)
	}

	def, err := image.ImageSource(platform).Marshal(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	if err := llb.WriteTo(def, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
