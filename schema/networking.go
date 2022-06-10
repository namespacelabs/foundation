// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

import (
	"fmt"
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
