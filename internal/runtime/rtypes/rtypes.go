// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package rtypes

import (
	"context"
	"io"
	"os"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	schema "namespacelabs.dev/foundation/schema"
)

type RunBinaryOpts struct {
	WorkingDir string
	Image      oci.Image
	Command    []string
	Args       []string
	Env        []*schema.BinaryConfig_EnvEntry
	RunAsUser  bool
}

type RunToolOpts struct {
	SupportedToolVersion int
	RunBinaryOpts
	IO
	ImageName    string
	MountAbsRoot string
	Mounts       []*LocalMapping
	AllocateTTY  bool
	NoNetworking bool // XXX remove, too specific.
}

type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func StdIO(ctx context.Context) IO {
	return IO{
		Stdin:  os.Stdin,
		Stdout: console.Stdout(ctx),
		Stderr: console.Stderr(ctx),
	}
}
