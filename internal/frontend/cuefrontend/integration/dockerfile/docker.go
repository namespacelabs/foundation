// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package dockerfile

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/entity"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func NewParser() entity.EntityParser {
	return &Parser{}
}

type cueIntegration struct {
	Src        string `json:"src"`
	WorkingDir string `json:"workingDir"`
	Command    string `json:"command"`
}

type Parser struct{}

func (p *Parser) Url() string      { return "namespace.so/from-dockerfile" }
func (p *Parser) Shortcut() string { return "dockerfile" }

func (p *Parser) Parse(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	if v != nil {
		if str, err := v.Val.String(); err == nil {
			// Shortcut: `dockerfile: "<filename>"`
			return &schema.DockerfileIntegration{Src: str}, nil
		}
	}

	var bits cueIntegration
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return nil, err
		}
	}

	out := &schema.DockerfileIntegration{
		Src:        bits.Src,
		WorkingDir: bits.WorkingDir,
	}

	if bits.Command != "" {
		out.Command = []string{bits.Command}
	}

	return out, nil
}
