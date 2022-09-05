// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

import "strings"

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
func ParsePackageRef(str string) (*PackageRef, error) {
	parts := strings.SplitN(str, ":", 2)

	pr := &PackageRef{}
	pr.PackageName = parts[0]
	if len(parts) > 1 {
		pr.Name = parts[1]
	}

	return pr, nil
}

func (n *PackageRef) AsPackageName() PackageName {
	return MakePackageName(n.PackageName)
}

func (n *PackageRef) Equals(other *PackageRef) bool {
	return n.Compare(other) == 0
}

func (n *PackageRef) Compare(other *PackageRef) int {
	if n.PackageName != other.PackageName {
		return strings.Compare(n.PackageName, other.PackageName)
	}
	return strings.Compare(n.Name, other.Name)
}

func (n *PackageRef) Canonical() string {
	if n.Name == "" {
		return n.PackageName
	}

	return n.PackageName + ":" + n.Name
}
