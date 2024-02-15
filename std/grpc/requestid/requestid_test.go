// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package requestid

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAttachRequestIDToError(t *testing.T) {
	withRid := AttachRequestIDToError(context.DeadlineExceeded, "some-rid")

	st, _ := status.FromError(withRid)
	if st.Code() != codes.DeadlineExceeded {
		t.Errorf("expected deadline exceeded, got code: %v", st.Code())
	}
}
