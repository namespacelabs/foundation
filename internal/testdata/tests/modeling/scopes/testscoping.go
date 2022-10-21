// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/testing"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/internal/testdata/service/count", "count")

		conn, err := t.Connect(ctx, endpoint)
		if err != nil {
			return err
		}

		cli := proto.NewCountServiceClient(conn)
		one, err := cli.Get(ctx, &proto.GetRequest{Name: "one"})
		if err != nil {
			return err
		}

		two, err := cli.Get(ctx, &proto.GetRequest{Name: "two"})
		if err != nil {
			return err
		}

		if _, err := cli.Increment(ctx, &proto.IncrementRequest{Name: "one"}); err != nil {
			return err
		}

		newone, err := cli.Get(ctx, &proto.GetRequest{Name: "one"})
		if err != nil {
			return err
		}

		newtwo, err := cli.Get(ctx, &proto.GetRequest{Name: "two"})
		if err != nil {
			return err
		}

		expected := one.Value + 1
		if expected != newone.Value {
			return fmt.Errorf("increment failed: expected %d but got %d", expected, newone.Value)
		}

		if two.Value != newtwo.Value {
			return fmt.Errorf("accidental side-effect: counter changed from %d to %d", two.Value, newtwo.Value)
		}

		if _, err := cli.Increment(ctx, &proto.IncrementRequest{Name: "unknown"}); err == nil {
			return fmt.Errorf("expected failure for Increment on unknown name")
		}

		if _, err := cli.Get(ctx, &proto.GetRequest{Name: "unknown"}); err == nil {
			return fmt.Errorf("expected failure for Increment on unknown name")
		}

		return nil
	})
}
