// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buf

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func MakeProtoSrcs(ctx context.Context, env ops.Environment, request map[schema.Framework]*protos.FileDescriptorSetAndDeps) (compute.Computable[fs.FS], error) {
	platform, err := tools.HostPlatform(ctx)
	if err != nil {
		return nil, err
	}

	keys := maps.Keys(request)
	slices.Sort(keys)

	base := State(platform)

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

		case schema.Framework_NODEJS:
			t.Plugins = append(t.Plugins,
				PluginTmpl{Name: "ts", Path: "protobuf-ts/packages/plugin/bin/protoc-gen-ts", Out: outDir, Opt: []string{
					// We generate gRPC wiring ourselves
					"force_disable_services",
					// Adding "_pb" suffix to differentiate from regular TS sources.
					"add_pb_suffix",
				}})

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
			llb.Network(llb.NetModeNone), llb.WithCustomNamef("generating %s proto sources", fmwk)) // protogen should not have network access.
		r.AddMount(srcDir, src, llb.Readonly)
		// The strategy here is to produce all results onto a directory structure that mimics the workspace,
		// but to a location off-workspace. This allow us to read the results into a snapshot without modifying
		// the workspace in-place. We can then decide to commit those results to the workspace.
		result := r.AddMount(outDir, llb.Scratch())
		out = out.File(llb.Copy(result, ".", "."), llb.WithCustomNamef("copying %s generated sources", fmwk))
	}

	img, err := buildkit.LLBToImage(ctx, env, build.NewBuildTarget(&platform).WithSourceLabel("protobuf-codegen"), out)
	if err != nil {
		return nil, err
	}

	return compute.Transform(img, func(ctx context.Context, img oci.Image) (fs.FS, error) {
		fsys := tarfs.FS{
			TarStream: func() (io.ReadCloser, error) {
				return mutate.Extract(img), nil
			},
		}

		return fsys, nil
	}), nil
}
