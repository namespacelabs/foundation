// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"testing"

	"gotest.tools/assert"
	localauth "namespacelabs.dev/foundation/internal/auth"
)

func TestFetchTokenFromContext(t *testing.T) {
	want := &localauth.Token{}

	got, err := FetchToken(WithToken(context.Background(), want))
	assert.NilError(t, err)
	assert.Assert(t, got == want)
}
