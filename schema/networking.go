// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func (e *Endpoint) GetServerOwnerPackage() PackageName {
	return PackageName(e.ServerOwner)
}

func (e *Endpoint) HasKind(str string) bool {
	for _, md := range e.ServiceMetadata {
		if md.GetKind() == str {
			return true
		}
	}
	return false
}

func (e *Endpoint) Address() string {
	return fmt.Sprintf("%s:%d", e.AllocatedName, e.GetPort().GetContainerPort())
}

type HasGetServiceMetadata interface {
	GetServiceMetadata() []*ServiceMetadata
}

func CombineServiceMetadata[V HasGetServiceMetadata](list []V) []*ServiceMetadata {
	var combined []*ServiceMetadata
	for _, m := range list {
		combined = append(combined, m.GetServiceMetadata()...)
	}
	return combined
}

func UnmarshalServiceMetadata[V proto.Message](mds []*ServiceMetadata, kind string) (V, error) {
	var empty V

	for _, md := range mds {
		if md.Kind == kind {
			if md.Details == nil {
				break
			}

			msg, err := md.Details.UnmarshalNew()
			if err != nil {
				return empty, err
			}

			v, ok := msg.(V)
			if !ok {
				return empty, fnerrors.InternalError("unexpected type %q", md.Details.TypeUrl)
			}

			return v, nil
		}
	}

	return empty, nil
}
