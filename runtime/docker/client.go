// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"

	configtypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

// Client implements the Docker client, but only with the bits that Foundation requires.
// It also performs Foundation-specific error handling
type Client interface {
	ServerVersion(ctx context.Context) (types.Version, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	ImageTag(ctx context.Context, source, target string) error
	Close() error
}

func NewClient() (Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	return wrappedClient{cli}, err
}

// From "github.com/docker/cli/cli/command", but avoiding dep creep.
func EncodeAuthToBase64(authConfig configtypes.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

type wrappedClient struct {
	cli *client.Client
}

func (w wrappedClient) ServerVersion(ctx context.Context) (types.Version, error) {
	v, err := w.cli.ServerVersion(ctx)
	return v, maybeReplaceErr(err)
}

func (w wrappedClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	v, err := w.cli.ContainerInspect(ctx, containerID)
	return v, maybeReplaceErr(err)
}

func (w wrappedClient) ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error {
	return maybeReplaceErr(w.cli.ContainerStart(ctx, containerID, options))
}

func (w wrappedClient) ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error {
	return maybeReplaceErr(w.cli.ContainerRemove(ctx, containerID, options))
}

func (w wrappedClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	i, b, err := w.cli.ImageInspectWithRaw(ctx, imageID)
	return i, b, maybeReplaceErr(err)
}

func (w wrappedClient) ImageLoad(ctx context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error) {
	v, err := w.cli.ImageLoad(ctx, input, quiet)
	return v, maybeReplaceErr(err)
}

func (w wrappedClient) ImageTag(ctx context.Context, source, target string) error {
	return maybeReplaceErr(w.cli.ImageTag(ctx, source, target))
}

func (w wrappedClient) Close() error {
	return maybeReplaceErr(w.cli.Close())
}

func maybeReplaceErr(err error) error {
	if errors.Is(err, os.ErrPermission) {
		var lines = []string{
			"Failed to connect to Docker, due to lack of permissions. This is likely",
			"due to your user not being in the right group to be able to use Docker.",
			"",
		}

		if runtime.GOOS == "linux" {
			lines = append(lines,
				"Checkout the following URL for instructions on how to handle this error:",
				"",
				"https://docs.docker.com/engine/install/linux-postinstall/")
		} else {
			lines = append(lines, "Please refer to Docker's documentation on how to solve this issue.")
		}

		return fnerrors.Wrapf(nil, err, strings.Join(lines, "\n"))
	}
	return err
}
