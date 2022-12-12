// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"namespacelabs.dev/foundation/framework/resources/provider"
	pb "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes/protos"
	"namespacelabs.dev/foundation/internal/testdata/integrations/resources/testgenprovider"
)

func main() {
	_, p := provider.MustPrepare[*testgenprovider.DatabaseIntent]()

	p.EmitResult(&pb.DatabaseInstance{Url: "http://test-" + p.Intent.Name})
}
