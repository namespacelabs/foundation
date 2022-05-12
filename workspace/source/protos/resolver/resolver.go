// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package resolver

import (
	"context"
	"path"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type providerProtoResolver struct {
	// proto resolver interface passes no context, so we retain the caller's.
	ctx  context.Context
	root workspace.Packages
}

func NewResolver(ctx context.Context, root workspace.Packages) *providerProtoResolver {
	return &providerProtoResolver{ctx, root}
}

func (pr *providerProtoResolver) resolvePackage(name schema.PackageName) (*workspace.Package, error) {
	return pr.root.LoadByName(pr.ctx, name)
}

func (pr *providerProtoResolver) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	return protoregistry.GlobalTypes.FindMessageByName(message)
}

func isBuiltinAny(url string) bool {
	return strings.HasPrefix(url, "type.googleapis.com/")
}

func (pr *providerProtoResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	if isBuiltinAny(url) {
		return protoregistry.GlobalTypes.FindMessageByURL(url)
	}

	v := strings.TrimPrefix(url, protos.TypeUrlBaseSlash)
	if v == url {
		return nil, protoregistry.NotFound
	}

	packageName := schema.PackageName(path.Dir(v))
	typeName := path.Base(v)

	pkg, err := pr.resolvePackage(packageName)
	if err != nil {
		return nil, err
	}

	for _, msg := range pkg.Provides {
		_, providermsg, err := protos.LoadMessageByName(msg, typeName)
		if err != nil {
			if err == protoregistry.NotFound {
				continue
			}
			return nil, err
		}

		return dynamicpb.NewMessageType(providermsg), nil
	}

	return nil, fnerrors.UserError(nil, "referenced node %s does not provide type %s", packageName, typeName)
}

func (pr *providerProtoResolver) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	return protoregistry.GlobalTypes.FindExtensionByName(field)
}

func (pr *providerProtoResolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return protoregistry.GlobalTypes.FindExtensionByNumber(message, field)
}
