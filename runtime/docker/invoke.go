// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/user"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/muesli/cancelreader"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type ToolRuntime struct{}

func Impl() ToolRuntime { return ToolRuntime{} }

func (r ToolRuntime) CanConsumePublicImages() bool {
	return false // This is not quite true but it's a simplification for now. // XXX support docker pull.
}

func (r ToolRuntime) Run(ctx context.Context, opts rtypes.RunToolOpts) error {
	return r.RunWithOpts(ctx, opts, nil)
}

func (r ToolRuntime) RunWithOpts(ctx context.Context, opts rtypes.RunToolOpts, onStart func()) error {
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
			return runImpl(ctx, opts, onStart)
		})
}

func HostPlatform() specs.Platform {
	p := devhost.RuntimePlatform()
	p.OS = "linux" // We always run on linux.
	return p
}

func (r ToolRuntime) HostPlatform(context.Context) (specs.Platform, error) {
	return HostPlatform(), nil
}

func runImpl(ctx context.Context, opts rtypes.RunToolOpts, onStart func()) error {
	var cmd []string
	cmd = append(cmd, opts.Command...)
	cmd = append(cmd, opts.Args...)

	containerConfig := &container.Config{
		WorkingDir:   opts.WorkingDir,
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

	for _, kv := range opts.Env {
		if kv.ExperimentalFromSecret != "" {
			return fnerrors.New("docker: doesn't support env.ExperimentalFromSecret")
		}

		containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", kv.Name, kv.Value))
	}

	hostConfig := &container.HostConfig{
		AutoRemove: true,
	}

	if opts.NoNetworking {
		hostConfig.NetworkMode = "none"
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

	computable, err := writeImageOnce(opts.ImageName, opts.Image)
	if err != nil {
		return err
	}

	config, err := compute.GetValue(ctx, computable)
	if err != nil {
		return err
	}

	containerConfig.Image = config.String()

	cli, err := NewClient()
	if err != nil {
		return err
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
			// If the docker daemon is already removing the container, because
			// e.g. it has returned from execution, then we may observe a
			// conflict with `removal of container XYZ is already in progress`.
			// We ignore that error here.
			if !client.IsErrNotFound(err) && !errdefs.IsConflict(err) {
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

	var errChs []chan error

	var stdin cancelreader.CancelReader
	if opts.Stdin != nil {
		inerr := make(chan error)
		errChs = append(errChs, inerr)

		stdin, err = cancelreader.NewReader(opts.Stdin)
		if err != nil {
			return err
		}

		go func() {
			defer close(inerr)

			// This would typically block forever, but we cancel the reader
			// below when the container returns. That path also handles
			// cancelation as the ContainerWait() call should observe
			// cancelation, which will then lead to canceling reads.
			if _, err := io.Copy(resp.Conn, stdin); err != nil {
				if !errors.Is(err, cancelreader.ErrCanceled) {
					inerr <- err
				}
				return
			}

			// If we reached expected EOF, signal that to the underlying container.
			if err := resp.CloseWrite(); err != nil {
				fmt.Fprintln(console.Errors(ctx), "Failed to close stdin", err)
			}
		}()
	}

	go func() {
		outerr := make(chan error)
		defer close(outerr)

		errChs = append(errChs, outerr)

		stdout := writerOrDiscard(opts.Stdout)
		stderr := writerOrDiscard(opts.Stderr)

		// Following Docker's implementation here. When AllocateTTY is set,
		// Docker multiplexes both output streams into stdout.
		if opts.AllocateTTY {
			_, err := io.Copy(stdout, resp.Reader)
			outerr <- err
		} else {
			_, err := stdcopy.StdCopy(stdout, stderr, resp.Reader)
			outerr <- err
		}
	}()

	if onStart != nil {
		// Signal OnStart after the various IO-related pipes started getting established.
		onStart()
	}

	waitErr := func() error {
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
	}()

	// Wait until the goroutines are done.
	goroutineErrs := make([]error, len(errChs))
	for i, errCh := range errChs {
		goroutineErrs[i] = <-errCh
	}

	if waitErr != nil {
		switch err := waitErr.(type) {
		case fnerrors.ExitError:
			if err.ExitCode() == 0 {
				return nil
			}
		}

		return waitErr
	}

	return multierr.New(goroutineErrs...)
}

func writerOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
