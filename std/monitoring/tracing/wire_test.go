// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tracing

import (
	"context"
	"testing"

	"namespacelabs.dev/foundation/std/core/types"
)

func TestCreateResource(t *testing.T) {
	_, err := CreateResource(context.Background(), &types.ServerInfo{ServerName: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}
