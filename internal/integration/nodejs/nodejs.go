// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
)

type Parser struct{}

func (i *Parser) Kind() string     { return "namespace.so/from-nodejs" }
func (i *Parser) Shortcut() string { return "nodejs" }

type cueIntegrationNodejs struct {
	Package string `json:"pkg"`
}

func (i *Parser) Parse(ctx context.Context, v *fncue.CueV) (proto.Message, error) {
	var bits cueIntegrationNodejs
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return nil, err
		}
	}

	return &schema.NodejsIntegration{
		Package: bits.Package,
	}, nil
}
