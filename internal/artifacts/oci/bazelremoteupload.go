// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/command"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/outerr"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/rexec"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

type bazelRemoteUploadLayer struct {
	v1.Layer
	executor *rexec.Client
}

var bazelRemoteUploadLayers sync.Map

func WithBazelRemoteUpload(layer Layer, executor *rexec.Client) (Layer, error) {
	remoteLayer := bazelRemoteUploadLayer{Layer: layer, executor: executor}
	// Image mutations may replace a layer object while retaining its content. Keep the
	// upload metadata keyed by the immutable compressed digest so it survives those mutations.
	digest, err := layer.Digest()
	if err != nil {
		return nil, fnerrors.Newf("failed to compute remote upload layer digest: %w", err)
	}
	bazelRemoteUploadLayers.Store(digest.String(), remoteLayer)
	return remoteLayer, nil
}

func remoteUploadLayer(layer v1.Layer) (bazelRemoteUploadLayer, bool, error) {
	if remoteLayer, ok := layer.(bazelRemoteUploadLayer); ok {
		return remoteLayer, true, nil
	}
	digest, err := layer.Digest()
	if err != nil {
		return bazelRemoteUploadLayer{}, false, fnerrors.Newf("failed to compute image layer digest: %w", err)
	}
	remoteLayer, ok := bazelRemoteUploadLayers.Load(digest.String())
	if !ok {
		return bazelRemoteUploadLayer{}, false, nil
	}
	return remoteLayer.(bazelRemoteUploadLayer), true, nil
}

func uploadBazelRemoteLayers(ctx context.Context, ref name.Tag, img v1.Image) error {
	if !isNamespaceRegistry(ref.Context().RegistryStr()) {
		return nil
	}

	layers, err := img.Layers()
	if err != nil {
		return err
	}
	for _, layer := range layers {
		remoteLayer, ok, err := remoteUploadLayer(layer)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := remoteLayer.upload(ctx, ref.Context()); err != nil {
			return err
		}
	}
	return nil
}

func isNamespaceRegistry(registry string) bool {
	return registry == "nscr.io" || strings.HasSuffix(registry, ".nscr.io")
}

func (layer bazelRemoteUploadLayer) upload(ctx context.Context, repository name.Repository) (retErr error) {
	digest, err := layer.Digest()
	if err != nil {
		return err
	}

	stagingDir, err := dirs.CreateUserTempDir("oci", "remote-upload")
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, os.RemoveAll(stagingDir))
	}()

	blob, err := os.Create(filepath.Join(stagingDir, "blob"))
	if err != nil {
		return err
	}
	compressed, err := layer.Compressed()
	if err != nil {
		return errors.Join(err, blob.Close())
	}
	_, copyErr := io.Copy(blob, compressed)
	if err := errors.Join(copyErr, compressed.Close(), blob.Close()); err != nil {
		return err
	}

	cmd := &command.Command{
		Args:     []string{"/bin/bash", "-c", remoteBlobUploadScript, "upload-registry-blob", repository.RegistryStr(), repository.RepositoryStr(), digest.String()},
		ExecRoot: stagingDir,
		InputSpec: &command.InputSpec{
			Inputs: []string{"blob"},
		},
		Timeout: 5 * time.Minute,
		Platform: map[string]string{
			"OSFamily":                   "Linux",
			"Arch":                       "amd64",
			"namespace_requires_network": "true",
		},
	}
	cmd.FillDefaultFieldValues()

	opts := command.DefaultExecutionOptions()
	// Registry state may outlive the RE action cache or be removed by registry GC.
	// Always execute the idempotent HEAD/upload operation rather than trusting an old action result.
	opts.DoNotCache = true
	oe := outerr.NewRecordingOutErr()
	result, _ := layer.executor.Run(ctx, cmd, opts, oe)
	if result.Err != nil {
		return fnerrors.Newf("remote registry blob upload failed: %w", result.Err)
	}
	if !result.IsOk() {
		return fnerrors.Newf("remote registry blob upload exited with status %d: %s", result.ExitCode, strings.TrimSpace(string(oe.Stderr())))
	}
	return nil
}

const remoteBlobUploadScript = `set -euo pipefail
registry="$1"
repository="$2"
digest="$3"
token="$(jq -r '.bearer_token' "${NSC_TOKEN_FILE:?NSC_TOKEN_FILE is not set}")"
auth=(--user "token:${token}")
base="https://${registry}/v2/${repository}/blobs"

if curl --fail --silent --location --head "${auth[@]}" "${base}/${digest}" >/dev/null 2>&1; then
  exit 0
fi

headers="$(mktemp)"
trap 'rm -f "$headers"' EXIT
curl --fail --silent --show-error --location --dump-header "$headers" --output /dev/null --request POST "${auth[@]}" "${base}/uploads/"
location="$(awk 'tolower($1) == "location:" { sub(/^[^:]*:[[:space:]]*/, ""); sub(/\r$/, ""); location=$0 } END { print location }' "$headers")"
test -n "$location"
case "$location" in
  http://*|https://*) ;;
  /*) location="https://${registry}${location}" ;;
  *) location="https://${registry}/${location}" ;;
esac
curl --fail --silent --show-error --location --dump-header "$headers" --output /dev/null --request PATCH "${auth[@]}" --header 'Content-Type: application/octet-stream' --data-binary @blob "$location"
location="$(awk 'tolower($1) == "location:" { sub(/^[^:]*:[[:space:]]*/, ""); sub(/\r$/, ""); location=$0 } END { print location }' "$headers")"
test -n "$location"
case "$location" in
  http://*|https://*) ;;
  /*) location="https://${registry}${location}" ;;
  *) location="https://${registry}/${location}" ;;
esac
case "$location" in
  *\?*) location="${location}&digest=${digest}" ;;
  *) location="${location}?digest=${digest}" ;;
esac
curl --fail --silent --show-error --location --request PUT "${auth[@]}" "$location"
`
