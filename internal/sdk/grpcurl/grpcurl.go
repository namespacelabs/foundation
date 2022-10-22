// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpcurl

import (
	"context"
	"fmt"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const version = "1.8.6"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: "https://github.com/fullstorydev/grpcurl/releases/download/v1.8.6/grpcurl_1.8.6_linux_x86_64.tar.gz",
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "5d6768248ea75b30fba09c09ff8ba91fbc0dd1a33361b847cdaf4825b1b514a7",
		},
	},
	"linux/arm64": {
		URL: "https://github.com/fullstorydev/grpcurl/releases/download/v1.8.6/grpcurl_1.8.6_linux_arm64.tar.gz",
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "8e68cef2b493e79ebf8cb6d867678cbba0b9c5dea75f238575fca4f3bcc539b2",
		},
	},
	"darwin/arm64": {
		URL: "https://github.com/fullstorydev/grpcurl/releases/download/v1.8.6/grpcurl_1.8.6_osx_arm64.tar.gz",
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "fe3ce63efb168e894f4af58512b1bd9e3327166f1616975a7dbb249a990ce6cf",
		},
	},
	"darwin/amd64": {
		URL: "https://github.com/fullstorydev/grpcurl/releases/download/v1.8.6/grpcurl_1.8.6_osx_x86_64.tar.gz",
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "f908d8d2006efaf702097593a2e030ddc9274c7d349b85bee9d3cfa099018854",
		},
	},
}

type Grpcurl string

func EnsureSDK(ctx context.Context) (Grpcurl, error) {
	sdk, err := SDK(ctx)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context) (compute.Computable[Grpcurl], error) {
	platform := devhost.RuntimePlatform()
	key := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.UserError(nil, "platform not supported: %s", key)
	}

	fsys := unpack.Unpack("grpcurl", tarfs.TarGunzip(download.URL(ref)))

	return compute.Map(
		tasks.Action("grpcurl.ensure").Arg("version", version).HumanReadablef("Ensuring grpcurl %s is installed", version),
		compute.Inputs().Computable("fsys", fsys),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Grpcurl, error) {
			unpacked := compute.MustGetDepValue(r, fsys, "fsys")
			return Grpcurl(filepath.Join(unpacked.Files, "grpcurl")), nil
		}), nil
}

func AllDownloads() []compute.Computable[bytestream.ByteStream] {
	var downloads []compute.Computable[bytestream.ByteStream]
	for _, v := range Pins {
		downloads = append(downloads, download.URL(v))
	}
	return downloads
}
