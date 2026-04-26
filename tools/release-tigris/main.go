// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Command release-tigris publishes the foundation ns/nsc release artifacts
// (and installer scripts) to a Tigris bucket. The actual upload logic lives in
// the standalone namespacelabs.dev/releaser module so the same code can be
// reused by other tools (devbox, etc.); this binary just wires foundation's
// defaults.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"namespacelabs.dev/releaser/publish"
	"namespacelabs.dev/releaser/tigris"
)

const foundationKeyPrefix = "foundation"

func main() {
	ctx := context.Background()

	if len(os.Args) < 2 {
		fatalf("usage: %s <release|installers> [flags]", os.Args[0])
	}

	switch os.Args[1] {
	case "release":
		if err := runRelease(ctx, os.Args[2:]); err != nil {
			fatalf("release upload failed: %v", err)
		}
	case "installers":
		if err := runInstallers(ctx, os.Args[2:]); err != nil {
			fatalf("installer upload failed: %v", err)
		}
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func runRelease(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	bucket := fs.String("bucket", "", "Tigris bucket name")
	endpoint := fs.String("endpoint", tigris.DefaultEndpoint, "Tigris S3 endpoint")
	distDir := fs.String("dist-dir", "dist", "GoReleaser dist directory")
	tag := fs.String("tag", "", "Release tag, for example v0.0.123")
	fs.Parse(args)

	return publish.Release(ctx, publish.ReleaseOptions{
		Bucket:    *bucket,
		Endpoint:  *endpoint,
		DistDir:   *distDir,
		Tag:       *tag,
		KeyPrefix: foundationKeyPrefix,
		Tools:     []string{"ns", "nsc"},
	})
}

func runInstallers(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("installers", flag.ExitOnError)
	bucket := fs.String("bucket", "", "Tigris bucket name")
	endpoint := fs.String("endpoint", tigris.DefaultEndpoint, "Tigris S3 endpoint")
	fs.Parse(args)

	return publish.Installers(ctx, publish.InstallerOptions{
		Bucket:    *bucket,
		Endpoint:  *endpoint,
		KeyPrefix: foundationKeyPrefix,
		Installers: map[string]string{
			"install/install.sh":     "install/install.sh",
			"install/install_nsc.sh": "install/install_nsc.sh",
		},
	})
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
