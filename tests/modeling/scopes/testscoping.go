// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/testdata/service/modeling"
	"namespacelabs.dev/foundation/testing"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/std/testdata/service/modeling", "modeling")

		conn, err := t.Connect(ctx, endpoint)
		if err != nil {
			return err
		}

		cli := modeling.NewModelingServiceClient(conn)
		res, err := cli.GetScopedData(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}

		if len(res.Item) != 2 {
			return fmt.Errorf("expected 2 items, got %d", len(res.Item))
		}
		path0 := strings.Join(res.Item[0].Data.Caller, "->")
		path1 := strings.Join(res.Item[1].Data.Caller, "->")

		if path0 == path1 {
			return fmt.Errorf("expected different caller paths, got twice:\n%s", path0)
		}
		return nil
	})
}
