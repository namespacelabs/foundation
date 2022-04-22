// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/std/testdata/service/awss3"
	"namespacelabs.dev/foundation/testing"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/std/testdata/service/awss3", "awss3")

		conn, err := t.Connect(ctx, endpoint)
		if err != nil {
			return err
		}

		cli := awss3.NewS3DemoServiceClient(conn)

		// This file is not present in the bucket.
		var resp *awss3.GetResponse
		if _, err = cli.Get(ctx, &awss3.GetRequest{
			Filename: "file which is not present",
		}); err == nil {
			return fmt.Errorf("calling on unexistent bucket should fail")
		}

		if _, err = cli.Add(ctx, &awss3.AddRequest{
			Filename: "foo",
			Contents: "bar",
		}); err != nil {
			return fmt.Errorf("Add failed with %v", err)
		}
		if resp, err = cli.Get(ctx, &awss3.GetRequest{
			Filename: "foo",
		}); err != nil {
			return fmt.Errorf("Get failed with %v", err)
		}
		if resp.Contents != "bar" {
			return fmt.Errorf("expected contents to be foocontent, got %s", resp.Contents)
		}

		// TODO make the test delete the bucket as well for cleanup.

		return nil
	})
}
