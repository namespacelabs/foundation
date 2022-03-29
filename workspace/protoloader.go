// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"fmt"
	"path"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

const typeUrlBaseSlash = "type.foundation.namespacelabs.dev/"

type providerProtoResolver struct {
	// proto resolver interface passes no context, so we retain the caller's.
	ctx  context.Context
	root Packages
}

func NewProviderProtoResolver(ctx context.Context, root Packages) *providerProtoResolver {
	return &providerProtoResolver{ctx, root}
}

func (pr *providerProtoResolver) resolvePackage(name schema.PackageName) (*Package, error) {
	return pr.root.LoadByName(pr.ctx, name)
}

func (pr *providerProtoResolver) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	return protoregistry.GlobalTypes.FindMessageByName(message)
}

func hasBuiltAnyTypeUrl(url string) bool {
	return strings.HasPrefix(url, "type.googleapis.com/")
}

func (pr *providerProtoResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	if hasBuiltAnyTypeUrl(url) {
		return protoregistry.GlobalTypes.FindMessageByURL(url)
	}

	v := strings.TrimPrefix(url, typeUrlBaseSlash)
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

func MarshalPackageAny(pkg schema.PackageName, msg proto.Message) (*anypb.Any, error) {
	typename := string(msg.ProtoReflect().Descriptor().FullName())

	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fnerrors.InternalError("%s: %s: failed to marshal message: %w", pkg, typename, err)
	}

	return &anypb.Any{
		TypeUrl: fmt.Sprintf("%s%s/%s", typeUrlBaseSlash, pkg, typename),
		Value:   msgBytes,
	}, nil
}

func FindProvider(pkg *Package, packageName schema.PackageName, typeName string) (*schema.Node, *schema.Provides) {
	// Only extensions can be providers.
	if n := pkg.Extension; n != nil {
		if packageName.Equals(n.GetPackageName()) {
			for _, p := range n.Provides {
				if p.Type.Typename == typeName {
					return n, p
				}
			}
		}
	}

	return nil, nil
}