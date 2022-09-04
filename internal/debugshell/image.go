// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debugshell

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/system"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
)

func BuildSpec() build.Spec { return debugShellBuild{} }

func Image(ctx context.Context, env planning.Context, platforms []specs.Platform, tag compute.Computable[oci.AllocatedName]) (compute.Computable[oci.ImageID], error) {
	prepared, err := multiplatform.PrepareMultiPlatformImage(ctx, env, build.Plan{
		SourceLabel: "debugshell.image",
		Spec:        BuildSpec(),
		Platforms:   platforms,
		PublishName: tag,
	})
	if err != nil {
		return nil, err
	}

	return oci.PublishResolvable(tag, prepared), nil
}

type debugShellBuild struct{}

var _ build.Spec = debugShellBuild{}

func (debugShellBuild) BuildImage(ctx context.Context, env planning.Context, conf build.Configuration) (compute.Computable[oci.Image], error) {
	image := llbutil.Image("ubuntu:20.04@sha256:8ae9bafbb64f63a50caab98fd3a5e37b3eb837a3e0780b78e5218e63193961f9", *conf.TargetPlatform())

	base := image.
		Run(llb.Shlexf("apt-get update")).
		Run(llb.Shlexf("apt-get install -y curl")).
		Run(llb.Shlexf("curl -L https://go.dev/dl/go1.17.8.%s-%s.tar.gz -o /tmp/go.tgz",
			conf.TargetPlatform().OS, conf.TargetPlatform().Architecture)).
		Run(llb.Shlexf("tar -C /usr/local -xzf /tmp/go.tgz")).
		Run(llb.Shlexf("rm /tmp/go.tgz"))

	gobase := base.
		AddEnv("CGO_ENABLED", "0").
		AddEnv("PATH", "/usr/local/go/bin:"+system.DefaultPathEnvUnix).
		AddEnv("GOPATH", "/go")

	r := gobase.Run(llb.Shlexf("go install github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.8.6"))

	return buildkit.LLBToImage(ctx, env, conf, r.Root())
}

func (debugShellBuild) PlatformIndependent() bool { return false }
