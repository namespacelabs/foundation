// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
)

type DockerIntegrationParser struct{}

func (i *DockerIntegrationParser) Kind() string     { return "namespace.so/from-dockerfile" }
func (i *DockerIntegrationParser) Shortcut() string { return "docker" }

type cueIntegrationDocker struct {
	Dockerfile string `json:"dockerfile"`
}

func (i *DockerIntegrationParser) Parse(ctx context.Context, v *fncue.CueV) (proto.Message, error) {
	var bits cueIntegrationDocker
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return nil, err
		}
	}

	return &schema.DockerIntegration{
		Dockerfile: bits.Dockerfile,
	}, nil
}
