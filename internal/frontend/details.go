// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package frontend

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
)

var (
	detailsTemplates map[string]proto.Message
	detailsFuncs     map[string]func(context.Context, ops.Environment, *schema.Server, proto.Message) (*rtypes.DetailsProps, error)
)

type PrepareProvisionFunc[V proto.Message] func(context.Context, ops.Environment, *schema.Server, V) (*rtypes.DetailsProps, error)

func RegisterDetails[V proto.Message](name string, tmpl proto.Message, f PrepareProvisionFunc[V]) {
	if detailsTemplates == nil {
		detailsTemplates = map[string]proto.Message{}
	}

	if detailsFuncs == nil {
		detailsFuncs = map[string]func(context.Context, ops.Environment, *schema.Server, proto.Message) (*rtypes.DetailsProps, error){}
	}

	detailsTemplates[name] = tmpl
	detailsFuncs[name] = func(ctx context.Context, env ops.Environment, s *schema.Server, m proto.Message) (*rtypes.DetailsProps, error) {
		return f(ctx, env, s, m.(V))
	}
}

func DetailsOf(name string) proto.Message {
	if tmpl, ok := detailsTemplates[name]; ok {
		return proto.Clone(tmpl)
	}

	return nil
}

func ProvisionDetails(ctx context.Context, name string, env ops.Environment, srv *schema.Server, m proto.Message) (*rtypes.DetailsProps, error) {
	if f, ok := detailsFuncs[name]; ok {
		return f(ctx, env, srv, m)
	}

	return nil, fnerrors.InternalError("%s: no registered handler", name)
}
