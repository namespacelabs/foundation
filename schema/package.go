// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

import "strings"

func NewPackageRef(pkg PackageName, name string) *PackageRef {
	return &PackageRef{
		PackageNameStr: pkg.String(),
		Name:           name,
	}
}

// Parses from a canonical string representation.
func ParsePackageRef(str string) (*PackageRef, error) {
	parts := strings.Split(str, ":")

	pr := &PackageRef{}
	pr.PackageNameStr = parts[0]
	if len(parts) > 1 {
		pr.Name = parts[1]
	}

	return pr, nil
}

func (n *PackageRef) PackageName() PackageName {
	return Name(n.PackageNameStr)
}

func (n *PackageRef) Equals(other *PackageRef) bool {
	return n.Compare(other) == 0
}

func (n *PackageRef) Compare(other *PackageRef) int {
	if n.PackageNameStr != other.PackageNameStr {
		return strings.Compare(n.PackageNameStr, other.PackageNameStr)
	}
	return strings.Compare(n.Name, other.Name)
}

func (n *PackageRef) CanonicalString() string {
	if n.Name == "" {
		return n.PackageNameStr
	}
	return n.PackageNameStr + ":" + n.Name
}
