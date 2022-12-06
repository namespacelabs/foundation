// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"log"
	"os"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/parsing/platform"
)

func main() {
	targetPlatform := os.Getenv("TARGET_PLATFORM")
	if targetPlatform == "" {
		log.Fatal("TARGET_PLATFORM is missing")
	}

	platform, err := platform.ParsePlatform(targetPlatform)
	if err != nil {
		log.Fatal(err)
	}

	def, err := makeMariaImageState(platform).Marshal(context.Background(), llb.Platform(platform))
	if err != nil {
		log.Fatal(err)
	}
	if err := llb.WriteTo(def, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
