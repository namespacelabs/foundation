// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"sort"

	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/framework/testing"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/internal/testdata/service/roachlist", "roachlist")

		conn, err := t.NewClient(endpoint)
		if err != nil {
			return err
		}

		cli := proto.NewListServiceClient(conn)

		items := []string{"item1", "item2"}

		for _, item := range items {
			if _, err = cli.Add(ctx, &proto.AddRequest{Item: item}); err != nil {
				return err
			}
		}

		resp, err := cli.List(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}

		if len(items) != len(resp.Item) {
			return fmt.Errorf("wrong list length: expected %d elements but got %d", len(items), len(resp.Item))
		}

		sort.Strings(resp.Item)
		for i, item := range items {
			if resp.Item[i] != item {
				return fmt.Errorf("item mismatch: '%v' is not '%v'", item, resp.Item[i])
			}
		}

		return nil
	})
}
