// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/system"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/pins"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	useSeparateGoModPhase = false
)

type buildConf interface {
	build.BuildTarget
	build.BuildWorkspace
}

func buildUsingBuildkit(ctx context.Context, env ops.Environment, bin GoBinary, conf buildConf) (compute.Computable[oci.Image], error) {
	local := buildkit.LocalContents{
		Module:         conf.Workspace(),
		Path:           bin.GoModulePath,
		ObserveChanges: bin.isFocus,
	}

	src := buildkit.MakeLocalState(local)

	base := makeGoBuildBase(ctx, bin.GoVersion, buildkit.HostPlatform())

	var prodBase llb.State

	if !bin.BinaryOnly {
		var err error
		prodBase, err = production.ServerImageLLB(production.Distroless, *conf.TargetPlatform())
		if err != nil {
			return nil, err
		}
	} else {
		prodBase = llb.Scratch()
	}

	label := "building"
	if bin.PackageName != "" {
		label += fmt.Sprintf(" %s", bin.PackageName)
	}

	goBuild := goBuildArgs(bin.GoVersion)
	goBuild = append(goBuild, fmt.Sprintf("-o=/out/%s", bin.BinaryName))

	state := (llbutil.RunGo{
		Base:       prepareGoMod(base, src, conf.TargetPlatform()).Root(),
		SrcMount:   src,
		WorkingDir: bin.SourcePath,
		Platform:   conf.TargetPlatform(),
	}).With(
		llbutil.PrefixSh(label, conf.TargetPlatform(), "go "+strings.Join(goBuild, " "))...).
		AddMount("/out", prodBase)

	image, err := buildkit.LLBToImage(ctx, env, conf, state, local)
	if err != nil {
		return nil, err
	}

	return compute.Named(
		tasks.Action("go.build.binary").Scope(bin.PackageName), image), nil
}

func prepareGoMod(base, src llb.State, platform *specs.Platform) llb.ExecState {
	r := llbutil.RunGo{
		Base:       base,
		SrcMount:   src,
		WorkingDir: ".",
		Platform:   platform,
	}

	ro := llbutil.PrefixSh("updating deps", platform, "go mod download -x")

	if git.AssumeSSHAuth {
		ro = append(ro, llb.AddSSHSocket(llb.SSHID(buildkit.SSHAgentProviderID), llb.SSHSocketTarget("/root/ssh-agent.sock")))
		ro = append(ro, llb.AddEnv("SSH_AUTH_SOCK", "/root/ssh-agent.sock"))
	}

	if !useSeparateGoModPhase {
		return r.With(ro...)
	}

	return r.PrepareGoMod(ro...)
}

func makeGoImage(version string) string {
	return fmt.Sprintf("docker.io/library/golang:%s-alpine", version)
}

func goAlpine(ctx context.Context, version string, platform specs.Platform) llb.State {
	img := makeGoImage(version)

	if r, err := pins.CheckImage(img); err == nil {
		return llbutil.Image(r, platform)
	}

	fmt.Fprintf(console.Warnings(ctx), "go: no pinned version of %q\n", img)

	return llbutil.Image(img, platform)
}

func makeGoBuildBase(ctx context.Context, version string, platform specs.Platform) llb.State {
	st := goAlpine(ctx, version, platform).
		AddEnv("CGO_ENABLED", "0").
		AddEnv("PATH", "/usr/local/go/bin:"+system.DefaultPathEnvUnix).
		AddEnv("GOPATH", "/go").
		Run(llb.Shlex("apk add --no-cache git openssh"),
			llb.WithCustomName("[prepare build image] apk add --no-cache git openssh")).Root()

	for _, ent := range git.NoPromptEnv() {
		st = st.AddEnv(ent[0], ent[1])
	}

	// Don't block builds on checking the pubkey of the target ssh host.
	// XXX security
	st = st.AddEnv("GIT_SSH_COMMAND", "ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no")

	if llbutil.GitCredentialsBuildkitSecret != "" {
		st = st.Run(llb.Shlex("git config --global credential.helper store")).Root()
	}

	return st
}
