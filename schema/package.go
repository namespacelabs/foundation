// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package schema

import (
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

func MakePackageSingleRef(pkg PackageName) *PackageRef {
	return &PackageRef{
		PackageName: pkg.String(),
	}
}

func MakePackageRef(pkg PackageName, name string) *PackageRef {
	return &PackageRef{
		PackageName: pkg.String(),
		Name:        name,
	}
}

// Parses from a canonical string representation.
func ParsePackageRef(owner PackageName, ref string) (*PackageRef, error) {
	if ref == "" {
		return nil, fnerrors.NewWithLocation(owner, "empty package refs are not permitted")
	}

	parts := strings.Split(ref, ":")

	if len(parts) > 2 {
		return nil, fnerrors.NewWithLocation(owner, "invalid package ref %q", ref)
	}

	pr := &PackageRef{}

	if parts[0] == "" {
		// Ref is of form ":foo", which implicitly references a name in the owning package.
		pr.PackageName = owner.String()
	} else {
		pr.PackageName = parts[0]
	}

	if len(parts) > 1 {
		pr.Name = parts[1]
	}

	return pr, nil
}

func StrictParsePackageRef(ref string) (*PackageRef, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return nil, fnerrors.New("invalid package ref %q", ref)
	}

	return &PackageRef{PackageName: parts[0], Name: parts[1]}, nil
}

func (n *PackageRef) AsPackageName() PackageName {
	return MakePackageName(n.GetPackageName())
}

func (n *PackageRef) Equals(other *PackageRef) bool {
	return n.Compare(other) == 0
}

func (n *PackageRef) Compare(other *PackageRef) int {
	if n.GetPackageName() != other.GetPackageName() {
		return strings.Compare(n.GetPackageName(), other.GetPackageName())
	}
	return strings.Compare(n.GetName(), other.GetName())
}

func (n *PackageRef) Canonical() string {
	if n.GetName() == "" {
		return n.GetPackageName()
	}

	return n.GetPackageName() + ":" + n.GetName()
}
