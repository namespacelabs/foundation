// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/schema/tasks"
)

func TestMultipleError(t *testing.T) {
	ctx := WithSink(context.Background(), nullSink{})
	af := Action("foobar").Start(ctx)
	_ = af.Done(multierr.New(errors.New("foobar0"), errors.New("foobar1")))

	stored := MakeStoreProto(&af.Data, nil)

	var messages []string
	var actionIDs []string
	for _, detail := range stored.ErrorDetails {
		actionID := &tasks.ErrorDetail_ActionID{}
		multi := &tasks.ErrorDetail_OriginalErrors{}
		if detail.MessageIs(multi) {
			if err := detail.UnmarshalTo(multi); err != nil {
				t.Fatal(err)
			}

			for _, e := range multi.Status {
				messages = append(messages, e.Message)
			}
		} else if detail.MessageIs(actionID) {
			if err := detail.UnmarshalTo(actionID); err != nil {
				t.Error(err)
			} else {
				actionIDs = append(actionIDs, actionID.ActionId)
			}
		} else {
			t.Errorf("unexpected detail: %v", detail.TypeUrl)
		}
	}

	if d := cmp.Diff([]string{
		"foobar0",
		"foobar1",
	}, messages); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}

	if d := cmp.Diff([]string{
		af.ID().String(),
	}, actionIDs); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}

}
