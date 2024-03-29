// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ServerFrameworkExt struct {
	Include           []schema.PackageName
	FrameworkSpecific *anypb.Any
}

type ServerInputs struct {
	Services []*schema.GrpcExportService
}

// XXX we're injection Location in these, which allows for loading arbitrary files for the workspace;
// Ideally we'd pass a PackageLoader instead.
type FrameworkHandler interface {
	PreParseServer(context.Context, pkggraph.Location, *ServerFrameworkExt) error
	PostParseServer(context.Context, *Sealed) error
	// List of packages that should be added as server dependencies, when the target environment's purpose is DEVELOPMENT.
	DevelopmentPackages() []schema.PackageName
}

type CueService struct {
	ProtoTypename string `json:"protoTypename"`
	GoPackage     string `json:"goPackage"`
}

var (
	FrameworkHandlers = map[schema.Framework]FrameworkHandler{}
)

func RegisterFrameworkHandler(framework schema.Framework, handler FrameworkHandler) {
	FrameworkHandlers[framework] = handler
}

func GetExtension(extensions []*anypb.Any, msg proto.Message) bool {
	for _, ext := range extensions {
		if ext.MessageIs(msg) {
			if ext.UnmarshalTo(msg) == nil {
				return true
			}
		}
	}

	return false
}

func MustExtension(extensions []*anypb.Any, msg proto.Message) error {
	if !GetExtension(extensions, msg) {
		return fnerrors.InternalError("didn't find required extension: %s", msg.ProtoReflect().Descriptor().FullName())
	}

	return nil
}
