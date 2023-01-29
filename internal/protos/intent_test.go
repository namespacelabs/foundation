// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package protos

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"namespacelabs.dev/foundation/schema"
)

var (
	//go:embed testdata/*.txt
	testData embed.FS
)

func TestAllocateMessage(t *testing.T) {
	for _, test := range []struct {
		JSON     string
		Expected proto.Message
	}{
		{
			JSON: `{
				"package_name": "xyz"
			}`,
			Expected: &schema.Server{PackageName: "xyz"},
		},
		{
			JSON: `{
				"packageName": "xyz"
			}`,
			Expected: &schema.Server{PackageName: "xyz"},
		},
		{
			JSON: `{
				"import": ["1", "2"],
				"main_container": {
					"binary_ref": {
						"package_name": "foobar"
					},
					"name": "sidecar",
					"args": ["a", "b"]
				}
			}`,
			Expected: &schema.Server{
				Import: []string{"1", "2"},
				MainContainer: &schema.Container{
					BinaryRef: &schema.PackageRef{
						PackageName: "foobar",
					},
					Name: "sidecar",
					Args: []string{"a", "b"},
				},
			},
		},
		{
			JSON: `{
				"inline": "testdata/fileresource.txt"
			}`,
			Expected: &schema.ConfigurableVolume_Entry{
				Inline: &schema.FileContents{
					Contents: []byte("This is test data."),
					Utf8:     true,
				},
			},
		},
		{
			JSON: `{
				"resource": ["testdata/fileresource.txt", "testdata/secondresource.txt"]
			}`,
			Expected: &schema.ResourceSet{
				Resource: []*schema.FileContents{
					{
						Contents: []byte("This is test data."),
						Utf8:     true,
					},
					{
						Contents: []byte("Another test."),
						Utf8:     true,
					},
				},
			},
		},
	} {
		msg, err := AllocateWellKnownMessage(context.Background(), ParseContext{FS: testData, SupportWellKnownMessages: true},
			test.Expected.ProtoReflect().Descriptor(), unmarshal(test.JSON))
		if err != nil {
			t.Error(err)
		} else {
			log.Printf("message: %+v", msg)

			if d := cmp.Diff(test.Expected, msg, protocmp.Transform()); d != "" {
				t.Errorf("mismatch (-want +got):\n%s", d)
			}
		}
	}
}

func unmarshal(str string) any {
	var m any

	if err := json.Unmarshal([]byte(str), &m); err != nil {
		panic(err)
	}

	return m
}
