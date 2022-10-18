// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"errors"
	"os"
	"testing"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
)

func TestErrorIsPropagated(t *testing.T) {
	// Ensure that an error in a computable is propagated to the waiter
	// and does not cancel all other computations.
	logLevel := 0
	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(os.Stdout, logLevel))
	expectedErr := fnerrors.ExpectedError("expected")

	if err := Do(ctx, func(ctx context.Context) error {
		c := Map(tasks.Action("test.error"),
			Inputs(),
			Output{NotCacheable: true},
			func(ctx context.Context, _ Resolved) (struct{}, error) {
				return struct{}{}, expectedErr
			})
		_, err := Get(ctx, c)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("unexpected error from computable: expected %v, got %v", expectedErr, err)
		}
		return nil
	}); err != nil {
		t.Fatal("error should not stop graph computation")
	}
}
