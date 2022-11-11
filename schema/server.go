// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package schema

func (x *Server) GetPackageRef() *PackageRef {
	if x == nil {
		return nil
	}

	return &PackageRef{
		PackageName: x.PackageName,
		Name:        x.Name,
	}
}
