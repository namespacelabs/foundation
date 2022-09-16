// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubetool

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/std/go/rpcerrors"
)

type perNode interface {
	CheckUnpackInput(msg proto.Message) (bool, error)
	UnpackInput(proto.Message) error
}

type ContextualEnv struct {
	Namespace       string
	CanSetNamespace bool // `ns` can set the namespace contextually.

	Context *KubernetesToolContext
}

// FromRequest is meant to be used by provisioning tools (and thus can os.Exit).
func FromRequest(r perNode) (*ContextualEnv, error) {
	var e ContextualEnv

	env := &KubernetesEnv{}
	if has, err := r.CheckUnpackInput(env); err != nil {
		return nil, err
	} else if has {
		e.Namespace = env.Namespace
	}

	ctx := &KubernetesToolContext{}
	if has, err := r.CheckUnpackInput(ctx); err != nil {
		return nil, err
	} else if has {
		e.Namespace = ctx.Namespace
		e.CanSetNamespace = ctx.CanSetNamespace
		e.Context = ctx
	}

	if e.Namespace == "" && !e.CanSetNamespace {
		return nil, rpcerrors.Errorf(codes.FailedPrecondition, "expected kubernetes context")
	}

	return &e, nil
}

func MustNamespace(r perNode) (*ContextualEnv, error) {
	res, err := FromRequest(r)
	if err != nil {
		return nil, err
	}

	if res.Namespace == "" {
		return nil, rpcerrors.Errorf(codes.FailedPrecondition, "kubernetes namespace missing")
	}

	return res, nil
}
