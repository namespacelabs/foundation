// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

	s3 "namespacelabs.dev/foundation/std/testdata/service/minio"
	"namespacelabs.dev/foundation/testing"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/std/testdata/service/minio", "minio")

		conn, err := t.Connect(ctx, endpoint)
		if err != nil {
			return err
		}

		cli := s3.NewS3DemoServiceClient(conn)

		// This file is not present in the bucket.
		if _, err = cli.Get(ctx, &s3.GetRequest{
			Filename: "file which is not present",
		}); err == nil {
			return fmt.Errorf("calling on unexistent bucket should fail")
		}

		if _, err = cli.Add(ctx, &s3.AddRequest{
			Filename: "foo",
			Contents: "bar",
		}); err != nil {
			return fmt.Errorf("add failed with %v", err)
		}

		resp, err := cli.Get(ctx, &s3.GetRequest{
			Filename: "foo",
		})
		if err != nil {
			return fmt.Errorf("get failed with %v", err)
		}

		if resp.Contents != "bar" {
			return fmt.Errorf("expected contents to be foocontent, got %s", resp.Contents)
		}

		return nil
	})
}
