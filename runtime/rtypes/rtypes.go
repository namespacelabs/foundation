// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package rtypes

import (
	"context"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"namespacelabs.dev/foundation/internal/console"
	schema "namespacelabs.dev/foundation/schema"
)

type RunBinaryOpts struct {
	WorkingDir string
	Image      v1.Image
	Command    []string
	Args       []string
	Env        []*schema.BinaryConfig_EnvEntry
	RunAsUser  bool
}

type RunToolOpts struct {
	RunBinaryOpts
	IO
	ImageName         string
	MountAbsRoot      string
	Mounts            []*LocalMapping
	AllocateTTY       bool
	NoNetworking      bool // XXX remove, too specific.
	UseHostNetworking bool // XXX remove, too specific.
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
