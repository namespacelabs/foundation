// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testing

import (
	"context"
	"io/fs"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/std/tasks"
)

func makeRequestDataLayer(testReq *testboot.TestRequest) oci.NamedLayer {
	return oci.MakeLayer("test-request-data",
		compute.Map(tasks.Action("test.make-request"),
			compute.Inputs().Proto("req", testReq).Str("basePath", testboot.TestRequestPath),
			compute.Output{},
			func(ctx context.Context, _ compute.Resolved) (fs.FS, error) {
				m, err := proto.MarshalOptions{Deterministic: true}.Marshal(testReq)
				if err != nil {
					return nil, err
				}

				var fsys memfs.FS
				fsys.Add(testboot.TestRequestPath, m)
				return &fsys, nil
			}))
}
