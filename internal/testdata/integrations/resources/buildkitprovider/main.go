// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"

	"namespacelabs.dev/foundation/framework/provisioning"
	pb "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes/protos"
	"namespacelabs.dev/foundation/internal/testdata/integrations/resources/testgenprovider"
)

// func main() {
// 	_, p := provider.MustPrepare[*testgenprovider.DatabaseIntent]()

// 	p.EmitResult(&pb.DatabaseInstance{Url: "http://buildkit-test-" + p.Intent.Name})
// }

func main() {
	h := provisioning.NewHandlers()
	henv := h.Any()
	henv.HandleApply(func(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
		intent := &testgenprovider.DatabaseIntent{}
		if err := req.UnpackInput(intent); err != nil {
			return err
		}

		out.OutputResourceInstance = &pb.DatabaseInstance{Url: "http://buildkit-test-" + intent.Name}
		return nil
	})
	provisioning.Handle(h)
}
