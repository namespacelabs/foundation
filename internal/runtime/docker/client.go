// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package docker

import (
	"bytes"
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
	"github.com/docker/go-connections/tlsconfig"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

// Client implements the Docker client, but only with the bits that Namespace requires.
// It also performs Namespace-specific error handling
type Client interface {
	ServerVersion(ctx context.Context) (ServerVersion, error)
	Info(ctx context.Context) (system.Info, error)
	ContainerCreate(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, string) (client.ContainerCreateResult, error)
	ContainerAttach(ctx context.Context, container string, options client.ContainerAttachOptions) (client.HijackedResponse, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) error
	ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error)
	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (io.ReadCloser, error)
	ImageTag(ctx context.Context, source, target string) error
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	ClientVersion() string
	Close() error
}

type ServerVersion struct {
	Platform      client.PlatformInfo       `json:",omitempty"`
	Version       string                    `json:",omitempty"`
	APIVersion    string                    `json:",omitempty"`
	MinAPIVersion string                    `json:",omitempty"`
	GitCommit     string                    `json:",omitempty"`
	GoVersion     string                    `json:",omitempty"`
	Os            string                    `json:",omitempty"`
	Arch          string                    `json:",omitempty"`
	KernelVersion string                    `json:",omitempty"`
	Experimental  bool                      `json:",omitempty"`
	BuildTime     string                    `json:",omitempty"`
	Components    []system.ComponentVersion `json:",omitempty"`
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

func (w wrappedClient) ServerVersion(ctx context.Context) (ServerVersion, error) {
	v, err := w.cli.ServerVersion(ctx, client.ServerVersionOptions{})
	converted := ServerVersion{
		Platform:      v.Platform,
		Version:       v.Version,
		APIVersion:    v.APIVersion,
		MinAPIVersion: v.MinAPIVersion,
		Os:            v.Os,
		Arch:          v.Arch,
		Experimental:  v.Experimental,
		Components:    v.Components,
	}
	return converted, maybeReplaceErr(err)
}

func (w wrappedClient) Info(ctx context.Context) (system.Info, error) {
	v, err := w.cli.Info(ctx, client.InfoOptions{})
	return v.Info, maybeReplaceErr(err)
}

func (w wrappedClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (client.ContainerCreateResult, error) {
	v, err := w.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		Platform:         platform,
		Name:             containerName,
	})
	return v, maybeReplaceErr(err)
}

func (w wrappedClient) ContainerAttach(ctx context.Context, container string, options client.ContainerAttachOptions) (client.HijackedResponse, error) {
	v, err := w.cli.ContainerAttach(ctx, container, options)
	return v.HijackedResponse, maybeReplaceErr(err)
}

func (w wrappedClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	v, err := w.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	return v.Container, maybeReplaceErr(err)
}

func (w wrappedClient) ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) error {
	_, err := w.cli.ContainerStart(ctx, containerID, options)
	return maybeReplaceErr(err)
}

func (w wrappedClient) ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) error {
	_, err := w.cli.ContainerRemove(ctx, containerID, options)
	return maybeReplaceErr(err)
}

func (w wrappedClient) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	// XXX we assume wrapping errors is not necessary here as ContainerWait is not used in isolation.
	res := w.cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{Condition: condition})
	return res.Result, res.Error
}

func (w wrappedClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	var raw bytes.Buffer
	i, err := w.cli.ImageInspect(ctx, imageID, client.ImageInspectWithRawResponse(&raw))
	return i.InspectResponse, raw.Bytes(), maybeReplaceErr(err)
}

func (w wrappedClient) ImageLoad(ctx context.Context, input io.Reader, quiet bool) (io.ReadCloser, error) {
	v, err := w.cli.ImageLoad(ctx, input, client.ImageLoadWithQuiet(quiet))
	return v, maybeReplaceErr(err)
}

func (w wrappedClient) ImageTag(ctx context.Context, source, target string) error {
	_, err := w.cli.ImageTag(ctx, client.ImageTagOptions{Source: source, Target: target})
	return maybeReplaceErr(err)
}

func (w wrappedClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	_, err := w.cli.VolumeRemove(ctx, volumeID, client.VolumeRemoveOptions{Force: force})
	return maybeReplaceErr(err)
}

func (w wrappedClient) ClientVersion() string {
	return w.cli.ClientVersion()
}

func (w wrappedClient) Close() error {
	return maybeReplaceErr(w.cli.Close())
}

func maybeReplaceErr(err error) error {
	switch {
	case errors.Is(err, os.ErrPermission):
		var lines = []string{
			"Failed to connect to Docker, due to lack of permissions. This is likely",
			"due to your user not being in the right group to be able to use Docker.",
		}

		var usage []string

		if runtime.GOOS == "linux" {
			usage = append(usage,
				"Checkout the following URL for instructions on how to handle this error:",
				"",
				"https://docs.docker.com/engine/install/linux-postinstall/")
		} else {
			usage = append(usage, "Please refer to Docker's documentation on how to solve this issue.")
		}

		return fnerrors.UsageError(strings.Join(usage, "\n"), "%s", strings.Join(lines, "\n"))

	case client.IsErrConnectionFailed(err):
		return fnerrors.UsageError("If you don't have Docker installed, please visit https://www.docker.com/get-started/", "unable to connect to Docker: %w", err)

	default:
		return err
	}
}
