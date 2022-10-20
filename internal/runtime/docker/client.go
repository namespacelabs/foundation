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
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	configtypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

// Client implements the Docker client, but only with the bits that Namespace requires.
// It also performs Namespace-specific error handling
type Client interface {
	ServerVersion(ctx context.Context) (types.Version, error)
	Info(ctx context.Context) (types.Info, error)
	ContainerCreate(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, string) (container.ContainerCreateCreatedBody, error)
	ContainerAttach(ctx context.Context, container string, options types.ContainerAttachOptions) (types.HijackedResponse, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	ImageTag(ctx context.Context, source, target string) error
	Close() error
}

func clientConfiguration() *Configuration {
	config := &Configuration{}
	fillConfigFromEnv(config)
	return config
}

func NewClient() (Client, error) {
	config := clientConfiguration()

	var opts []client.Opt

	helper, err := connhelper.GetConnectionHelper(config.Host)
	if err != nil {
		return nil, err
	}

	if helper == nil {
		opts = append(opts, client.WithHost(config.Host))
	} else {
		httpClient := &http.Client{
			// No tls
			// No proxy
			Transport: &http.Transport{
				DialContext: helper.Dialer,
			},
		}
		opts = append(opts,
			client.WithHTTPClient(httpClient),
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		)
	}

	opts = append(opts, client.WithAPIVersionNegotiation())

	if config.CertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(config.CertPath, "ca.pem"),
			CertFile:           filepath.Join(config.CertPath, "cert.pem"),
			KeyFile:            filepath.Join(config.CertPath, "key.pem"),
			InsecureSkipVerify: !config.VerifyTls,
		}
		tlsc, err := tlsconfig.Client(options)
		if err != nil {
			return nil, err
		}

		httpClient := &http.Client{
			Transport:     &http.Transport{TLSClientConfig: tlsc},
			CheckRedirect: client.CheckRedirect,
		}

		opts = append(opts, client.WithHTTPClient(httpClient))
	}

	if config.Version != "" {
		opts = append(opts, client.WithVersion(config.Version))
	}

	cli, err := client.NewClientWithOpts(opts...)
	return wrappedClient{cli}, err
}

func fillConfigFromEnv(config *Configuration) {
	config.Version = os.Getenv("DOCKER_API_VERSION")
	config.CertPath = os.Getenv("DOCKER_CERT_PATH")
	config.VerifyTls = os.Getenv("DOCKER_TLS_VERIFY") != ""
	config.Host = os.Getenv("DOCKER_HOST")

	if config.Host == "" {
		config.Host = client.DefaultDockerHost
	}
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

func (w wrappedClient) Info(ctx context.Context) (types.Info, error) {
	v, err := w.cli.Info(ctx)
	return v, maybeReplaceErr(err)
}

func (w wrappedClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.ContainerCreateCreatedBody, error) {
	v, err := w.cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, platform, containerName)
	return v, maybeReplaceErr(err)
}

func (w wrappedClient) ContainerAttach(ctx context.Context, container string, options types.ContainerAttachOptions) (types.HijackedResponse, error) {
	v, err := w.cli.ContainerAttach(ctx, container, options)
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

func (w wrappedClient) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error) {
	// XXX we assume wrapping errors is not necessary here as ContainerWait is not used in isolation.
	return w.cli.ContainerWait(ctx, containerID, condition)
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
