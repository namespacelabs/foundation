// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integrations

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
)

type GoIntegrationParser struct{}

func (i *GoIntegrationParser) Kind() string     { return "namespace.so/from-go" }
func (i *GoIntegrationParser) Shortcut() string { return "go" }

type cueIntegrationGo struct {
	Package string `json:"pkg"`
}

func (i *GoIntegrationParser) Parse(ctx context.Context, v *fncue.CueV) (proto.Message, error) {
	var bits cueIntegrationGo
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return nil, err
		}
	}

	return &schema.GoIntegration{
		Package: bits.Package,
	}, nil
}
