// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/muesli/cancelreader"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type ToolRuntime struct{}

func Impl() ToolRuntime { return ToolRuntime{} }

func (r ToolRuntime) Run(ctx context.Context, opts rtypes.RunToolOpts) error {
	return r.RunWithOpts(ctx, opts, localexec.RunOpts{})
}

func (r ToolRuntime) RunWithOpts(ctx context.Context, opts rtypes.RunToolOpts, additional localexec.RunOpts) error {
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
			return runImpl(ctx, opts, additional)
		})
}

func HostPlatform() specs.Platform {
	p := devhost.RuntimePlatform()
	p.OS = "linux" // We always run on linux.
	return p
}

func (r ToolRuntime) HostPlatform() specs.Platform { return HostPlatform() }

func runImpl(ctx context.Context, opts rtypes.RunToolOpts, additional localexec.RunOpts) error {
	computable, err := writeImageOnce(opts.ImageName, opts.Image)
	if err != nil {
		return err
	}

	config, err := compute.GetValue(ctx, computable)
	if err != nil {
		return err
	}

	cli, err := NewClient()
	if err != nil {
		return err
	}

	var cmd []string
	cmd = append(cmd, opts.Command...)
	cmd = append(cmd, opts.Args...)

	containerConfig := &container.Config{
		WorkingDir:   opts.WorkingDir,
		Image:        config.String(),
		Tty:          opts.AllocateTTY,
		AttachStdout: true, // Stdout, Stderr is always attached, even if discarded later (see below).
		AttachStderr: true,
		Cmd:          strslice.StrSlice(cmd),
	}

	if opts.Stdin != nil {
		containerConfig.AttachStdin = true
		containerConfig.OpenStdin = true
		// After we're done with Attach, the container should observe a EOF on Stdin.
		containerConfig.StdinOnce = true
	}

	if opts.RunAsUser {
		uid, err := user.Current()
		if err != nil {
			return err
		}

		containerConfig.User = fmt.Sprintf("%s:%s", uid.Uid, uid.Gid)
	}

	for k, v := range opts.Env {
		containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", k, v))
	}

	hostConfig := &container.HostConfig{
		AutoRemove: true,
	}

	if opts.NoNetworking {
		hostConfig.NetworkMode = "none"
	} else if opts.UseHostNetworking {
		hostConfig.NetworkMode = "host"
	}

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

		hostConfig.Binds = append(hostConfig.Binds, absPath+":"+m.ContainerPath)
	}

	networkConfig := &network.NetworkingConfig{}

	created, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, "")
	if err != nil {
		return err
	}

	fmt.Fprintf(console.Debug(ctx), "docker: created container %q (image=%s args=%v)\n",
		created.ID, containerConfig.Image, containerConfig.Cmd)

	compute.On(ctx).Cleanup(tasks.Action("docker.container.remove"), func(ctx context.Context) error {
		if err := cli.ContainerRemove(ctx, created.ID, types.ContainerRemoveOptions{}); err != nil {
			if !client.IsErrNotFound(err) {
				return err
			}
		}
		return nil
	})

	resp, err := cli.ContainerAttach(ctx, created.ID, types.ContainerAttachOptions{
		Stream: true,
		Stdin:  containerConfig.AttachStdin,
		Stdout: containerConfig.AttachStdout,
		Stderr: containerConfig.AttachStderr,
	})
	if err != nil {
		return err
	}

	if err := cli.ContainerStart(ctx, created.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	stdout := writerOrDiscard(opts.Stdout)
	stderr := writerOrDiscard(opts.Stderr)

	ex, wait := executor.New(ctx)

	ex.Go(func(ctx context.Context) error {
		// Following Docker's implementation here. When AllocateTTY is set,
		// Docker multiplexes both output streams into stdout.
		if opts.AllocateTTY {
			_, err := io.Copy(stdout, resp.Reader)
			return err
		} else {
			_, err := stdcopy.StdCopy(stdout, stderr, resp.Reader)
			return err
		}
	})

	var stdin cancelreader.CancelReader
	if opts.Stdin != nil {
		stdin, err = cancelreader.NewReader(opts.Stdin)
		if err != nil {
			return err
		}

		ex.Go(func(ctx context.Context) error {
			// This would typically block forever, but we cancel the reader
			// below when the container returns. That path also handles
			// cancelation as the ContainerWait() call should observe
			// cancelation, which will then lead to canceling reads.
			if _, err := io.Copy(resp.Conn, stdin); err != nil {
				if errors.Is(err, cancelreader.ErrCanceled) {
					return nil
				}

				return err
			}

			// If we reached expected EOF, signal that to the underlying container.
			if err := resp.CloseWrite(); err != nil {
				fmt.Fprintln(console.Errors(ctx), "Failed to close stdin", err)
			}

			return nil
		})
	}

	if additional.OnStart != nil {
		// Signal OnStart after the various IO-related pipes started getting established.
		additional.OnStart()
	}

	ex.Go(func(ctx context.Context) error {
		if stdin != nil {
			// Very important to cancel stdin when we're done, else we'll block forever.
			defer stdin.Cancel()
		}

		// After we're done waiting, we close the connection, which will lead
		// the stdout/stderr goroutine to exit.
		defer resp.Close()

		results, errs := cli.ContainerWait(ctx, created.ID, container.WaitConditionNextExit)
		select {
		case result := <-results:
			// An error is used to signal the parent in order to comply with the
			// Executor protocol. We want the first error to be recorded as
			// primary, and in this case that would be the observed exit code.
			// If for example we fail to read from a stream after observing exit
			// code 0, we should still not return an error.
			return fnerrors.ExitWithCode(fmt.Errorf("docker: container exit code %d", result.StatusCode), int(result.StatusCode))
		case err := <-errs:
			return err
		}
	})

	if err := wait(); err != nil {
		if exitErr, ok := err.(fnerrors.ExitError); !ok || exitErr.ExitCode() != 0 {
			return err
		}
	}

	return nil
}

func writerOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

func DockerRun(ctx context.Context, args []string, opts rtypes.IO) error {
	return dockerRun(ctx, args, opts, localexec.RunOpts{})
}

func dockerRun(ctx context.Context, args []string, opts rtypes.IO, additional localexec.RunOpts) error {
	return tasks.Action("docker.run").
		LogLevel(2).
		Arg("args", args).
		Run(ctx, func(ctx context.Context) error {
			c := exec.CommandContext(ctx, "docker", args...)

			c.Stdin = opts.Stdin
			c.Stdout = opts.Stdout
			c.Stderr = opts.Stderr
			c.Env = []string{}
			c.Env = append(c.Env, os.Environ()...)
			c.Env = append(c.Env, clientConfiguration().asEnv()...)

			return localexec.RunAndPropagateCancelationWithOpts(ctx, "docker", c, additional)
		})
}
