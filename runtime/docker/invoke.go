// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type ToolRuntime struct{}

func Impl() ToolRuntime { return ToolRuntime{} }

func (r ToolRuntime) Run(ctx context.Context, opts rtypes.RunToolOpts) error {
	digest, err := opts.Image.Digest()
	if err != nil {
		return err
	}

	config, err := opts.Image.ConfigName()
	if err != nil {
		return err
	}

	return tasks.Action("container.execute").
		LogLevel(2).
		Arg("command", opts.Command).
		Arg("imageName", opts.ImageName).
		Arg("digest", digest).
		Arg("config", config).
		Arg("args", opts.Args).
		Run(ctx, func(ctx context.Context) error {
			return runImpl(ctx, opts)
		})
}

func HostPlatform() specs.Platform {
	p := devhost.RuntimePlatform()
	p.OS = "linux" // We always run on linux.
	return p
}

func (r ToolRuntime) HostPlatform() specs.Platform { return HostPlatform() }

func runImpl(ctx context.Context, opts rtypes.RunToolOpts) error {
	n := opts.ImageName
	if n == "" {
		n = "foundation.namespacelabs.dev/docker-invocation"
	}

	tag, err := name.NewTag(n, name.WithDefaultTag("local"))
	if err != nil {
		return err
	}

	config, err := opts.Image.ConfigName()
	if err != nil {
		return fnerrors.RemoteError("docker: failed to fetch image config: %w", err)
	}

	if err := WriteImage(ctx, opts.Image, tag, false); err != nil {
		return err
	}

	// We don't pull with `docker run` to not interfere with os.Stderr.
	args := []string{"run", "--rm", "--pull=never"}

	if opts.Stdin != nil {
		args = append(args, "-i")

		if opts.AllocateTTY {
			args = append(args, "-t")
		}
	}

	if opts.WorkingDir != "" {
		args = append(args, "-w", opts.WorkingDir)
	}

	if opts.NoNetworking {
		args = append(args, "--net=none")
	} else if opts.UseHostNetworking {
		args = append(args, "--net=host")
	}

	// XXX Workspace maps should be approved by the user?
	for _, m := range opts.Mounts {
		var absPath string

		if m.HostPath != "" {
			if !filepath.IsAbs(m.HostPath) {
				return fnerrors.UserError(nil, "host_path must be absolute, got %q", m.HostPath)
			}
			absPath = m.HostPath
		} else {
			if opts.MountAbsRoot == "" {
				return fnerrors.InternalError("container.exec: LocalPath mount without MountAbsRoot")
			}

			absPath = filepath.Join(opts.MountAbsRoot, m.LocalPath)
			if _, err := filepath.Rel(opts.MountAbsRoot, absPath); err != nil {
				return err
			}
		}
		args = append(args, "-v", absPath+":"+m.ContainerPath)
	}

	for _, p := range opts.PortMap {
		args = append(args, "-p", fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort))
	}

	if opts.Env != nil {
		for k, v := range opts.Env {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
	}

	if opts.RunAsUser {
		uid, err := user.Current()
		if err != nil {
			return err
		}

		args = append(args, fmt.Sprintf("--user=%s:%s", uid.Uid, uid.Gid))
	}

	args = append(args, config.String())
	args = append(args, opts.Command...)
	args = append(args, opts.Args...)

	return DockerRun(ctx, args, opts.IO)
}

func DockerRun(ctx context.Context, args []string, opts rtypes.IO) error {
	return tasks.Action("docker.run").
		LogLevel(2).
		Arg("args", args).
		Run(ctx, func(ctx context.Context) error {
			c := exec.CommandContext(ctx, "docker", args...)

			c.Stdin = opts.Stdin
			c.Stdout = opts.Stdout
			c.Stderr = opts.Stderr

			return localexec.RunAndPropagateCancelation(ctx, "docker", c)
		})
}