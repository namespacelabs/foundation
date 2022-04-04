// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
	"sort"

	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/testdata/service/multidb"
	"namespacelabs.dev/foundation/testing"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/std/testdata/service/multidb", "multidb")

		conn, err := t.Connect(ctx, endpoint)
		if err != nil {
			return err
		}

		cli := multidb.NewListServiceClient(conn)

		if _, err = cli.AddPostgres(ctx, &multidb.AddRequest{Item: "postgres-item"}); err != nil {
			return err
		}
		if _, err = cli.AddMaria(ctx, &multidb.AddRequest{Item: "maria-item"}); err != nil {
			return err
		}

		resp, err := cli.List(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}

		sort.Strings(resp.Item)

		expected := []string{"maria-item", "postgres-item"}
		if len(expected) != len(resp.Item) {
			return fmt.Errorf("wrong list length: expected %d elements but got %d", len(expected), len(resp.Item))
		}

		for i, item := range expected {
			if resp.Item[i] != item {
				return fmt.Errorf("item mismatch: '%v' is not '%v'", item, resp.Item[i])
			}
		}

		return nil
	})
}
