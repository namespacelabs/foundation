// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shellscript

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/entity"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// Custom type as CUE -> Proto decoding does not accept camel-case syntax.
type cueShellScriptIntegration struct {
	Entrypoint       string   `json:"entrypoint,omitempty"`
	RequiredPackages []string `json:"requiredPackages,omitempty"`
}

type parser struct {
}

func (p *parser) Url() string      { return "namespace.so/from-shellscript" }
func (p *parser) Shortcut() string { return "shellscript" }

func (p *parser) Parse(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	var msg cueShellScriptIntegration
	if v != nil {
		if err := v.Val.Decode(&msg); err != nil {
			return nil, err
		}
	}

	return &schema.ShellScriptIntegration{
		Entrypoint:       msg.Entrypoint,
		RequiredPackages: msg.RequiredPackages,
	}, nil
}

func NewParser() entity.EntityParser {
	return &parser{}
}
