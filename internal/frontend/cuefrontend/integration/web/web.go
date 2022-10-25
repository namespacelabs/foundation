// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package web

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/nodejs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type Parser struct{}

func (i *Parser) Url() string      { return "namespace.so/from-web" }
func (i *Parser) Shortcut() string { return "web" }

type cueIntegrationWeb struct {
	// The service that corresponds to this web integration.
	// Needed to get the port for prod serving.
	Service string `json:"service"`
}

func (i *Parser) Parse(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	nodejsParser := &nodejs.Parser{}

	rawNodejsBuild, err := nodejsParser.Parse(ctx, env, pl, loc, v)
	if err != nil {
		return nil, err
	}

	nodejsBuild, ok := rawNodejsBuild.(*schema.NodejsBuild)
	if !ok {
		return nil, fnerrors.InternalError("expected nodejs integration")
	}

	if nodejsBuild.Prod != nil {
		nodejsBuild.Prod.InstallDeps = false

		if nodejsBuild.Prod.BuildOutDir == "" {
			nodejsBuild.Prod.BuildOutDir = "dist"
		}

		if nodejsBuild.Prod.BuildScript == "" {
			return nil, fnerrors.UserError(loc, "The `build` script is required for prod web build")
		}
	}

	var bits cueIntegrationWeb
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return nil, err
		}
	}

	if bits.Service == "" {
		return nil, fnerrors.UserError(loc, "web integration requires a service name")
	}

	return &schema.WebIntegration{
		Nodejs:  nodejsBuild,
		Service: bits.Service,
	}, nil
}
