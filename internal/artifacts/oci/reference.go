// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import "github.com/google/go-containerregistry/pkg/name"

type imageReference struct {
	name.Reference
	repository name.Repository
}

func (i *imageReference) Context() name.Repository {
	return i.repository
}
