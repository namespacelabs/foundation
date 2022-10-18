// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	Build cueIntegrationWebBuild `json:"build"`

	// The service that corresponds to this web integration.
	// Needed to get the port for prod serving.
	Service string `json:"service"`

	// Name -> package name.
	// The ingress urls for backends are injected into the built image as a JS file.
	Backends map[string]string `json:"backends"`
}

type cueIntegrationWebBuild struct {
	OutDir string `json:"outDir"`
}

func (i *Parser) Parse(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	nodejsParser := &nodejs.Parser{}

	rawNodejsInt, err := nodejsParser.Parse(ctx, pl, loc, v)
	if err != nil {
		return nil, err
	}

	nodejsInt, ok := rawNodejsInt.(*schema.NodejsIntegration)
	if !ok {
		return nil, fnerrors.InternalError("expected nodejs integration")
	}

	var bits cueIntegrationWeb
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return nil, err
		}
	}

	for k, v := range bits.Backends {
		serviceRef, err := schema.ParsePackageRef(loc.PackageName, v)
		if err != nil {
			return nil, err
		}

		nodejsInt.Backend = append(nodejsInt.Backend, &schema.NodejsIntegration_Backend{
			Name:    k,
			Service: serviceRef,
		})
	}

	if bits.Service == "" {
		return nil, fnerrors.UserError(loc, "web integration requires a service name")
	}

	if bits.Build.OutDir == "" {
		bits.Build.OutDir = "dist"
	}

	return &schema.WebIntegration{
		Nodejs:         nodejsInt,
		BuildOutputDir: bits.Build.OutDir,
		Service:        bits.Service,
	}, nil
}
