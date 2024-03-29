// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buf

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func MakeProtoSrcs(ctx context.Context, conf cfg.Configuration, request map[schema.Framework]*protos.FileDescriptorSetAndDeps) (compute.Computable[fs.FS], error) {
	cli, err := buildkit.Client(ctx, conf, nil)
	if err != nil {
		return nil, err
	}

	keys := maps.Keys(request)
	slices.Sort(keys)

	hostPlatform := cli.BuildkitOpts().HostPlatform
	base, err := State(ctx, hostPlatform)
	if err != nil {
		return nil, err
	}

	out := llb.Scratch()
	for _, fmwk := range keys {
		if len(request[fmwk].File) == 0 {
			continue
		}

		t := GenerateTmpl{
			Version: "v1",
		}

		// These directories are used within the container. Both will be mapped to temp dirs in the host
		// which are managed below, and will be deleted on completion.
		const outDir = "/out"
		const srcDir = "/src"

		switch fmwk {
		case schema.Framework_GO:
			t.Plugins = append(t.Plugins,
				PluginTmpl{Name: "go", Out: outDir, Opt: []string{"paths=source_relative"}},
				PluginTmpl{Name: "go-grpc", Out: outDir, Opt: []string{"paths=source_relative", "require_unimplemented_servers=false"}})

		default:
			return nil, fnerrors.BadInputError("%s: framework not supported", fmwk)
		}

		templateBytes, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}

		// Make all files available to buf, but then constrain below which files we request
		// generation on.
		fdsBytes, err := proto.Marshal(request[fmwk].AsFileDescriptorSet())
		if err != nil {
			return nil, err
		}

		const srcfile = "image.bin"
		src := llb.Scratch().File(llb.Mkfile(srcfile, 0600, fdsBytes))

		args := []string{"buf", "generate", "--template", string(templateBytes), filepath.Join(srcDir, srcfile)}

		for _, p := range request[fmwk].File {
			args = append(args, "--path", p.GetName())
		}

		r := base.Run(
			llb.Args(args),
			// This target doesn't exist yet. But it already prevents generating `google/protobuf/descriptor_pb.ts`
			// just for `std/proto/options.proto` which is not referenced by the generated code.
			llb.AddEnv("PROTOBUF_TS_RUNTIME_WELL_KNOWN_TYPES_IMPORT_PATH", "@namespacelabs/fn-protos"),
			llb.Network(llb.NetModeNone), llb.WithCustomNamef("generating %s proto sources", fmwk)) // protogen should not have network access.
		r.AddMount(srcDir, src, llb.Readonly)
		// The strategy here is to produce all results onto a directory structure that mimics the workspace,
		// but to a location off-workspace. This allow us to read the results into a snapshot without modifying
		// the workspace in-place. We can then decide to commit those results to the workspace.
		result := r.AddMount(outDir, llb.Scratch())
		out = out.File(llb.Copy(result, ".", "."), llb.WithCustomNamef("copying %s generated sources", fmwk))
	}

	return buildkit.BuildFilesystem(ctx, cli, build.NewBuildTarget(&hostPlatform).WithSourceLabel("protobuf-codegen"), out)
}
