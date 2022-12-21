// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"

	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/library/oss/localstack"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	h := provisioning.NewHandlers()
	henv := h.Any()
	henv.HandleApply(func(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
		intent := &localstack.ServerIntent{}
		if err := req.UnpackInput(intent); err != nil {
			return err
		}

		srv := intent.Server
		if srv == nil {
			srv = schema.MakePackageSingleRef("namespacelabs.dev/foundation/library/oss/localstack/server")
		}

		out.ComputedResourceInput = append(out.ComputedResourceInput, provisioning.ResourceInput{
			Name:   "server",
			Class:  schema.MakePackageRef("namespacelabs.dev/foundation/library/runtime", "Server"),
			Intent: srv,
		})

		return nil
	})
	provisioning.Handle(h)
}
