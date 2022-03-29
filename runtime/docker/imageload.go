// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/ctxio"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func WriteImage(ctx context.Context, img v1.Image, ref name.Tag, ensureTag bool) error {
	digest, err := img.Digest()
	if err != nil {
		return err
	}

	config, err := img.ConfigName()
	if err != nil {
		return err
	}

	return tasks.Action("docker.image-load").Arg("ref", ref).Arg("digest", digest).Arg("config", config).Run(ctx, func(ctx context.Context) error {
		client, err := NewClient()
		if err != nil {
			return err
		}

		if inspect, _, err := client.ImageInspectWithRaw(ctx, config.String()); err == nil {
			if !ensureTag {
				return nil
			}

			if slices.Contains(inspect.RepoTags, ref.String()) {
				// Nothing to do.
				return nil
			}

			if err := client.ImageTag(ctx, config.String(), ref.String()); err == nil {
				tasks.Attachments(ctx).AddResult("tagged", true)
				return nil
			}

			// If tagging fails, lets try to re-upload.
		}

		if _, err := writeImage(ctx, client, ref, img); err != nil {
			return fnerrors.RemoteError("failed to push to docker: %w", err)
		}

		tasks.Attachments(ctx).AddResult("uploaded", true)

		return nil
	})
}

// Write saves the image into the daemon as the given tag.
func writeImage(ctx context.Context, client *client.Client, tag name.Tag, img v1.Image) (string, error) {
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(tarball.Write(tag, img, ctxio.WriterWithContext(ctx, pw, nil)))
	}()

	progressReader := artifacts.NewProgressReader(pr, 0)
	tasks.Attachments(ctx).SetProgress(progressReader)

	// write the image in docker save format first, then load it
	resp, err := client.ImageLoad(ctx, progressReader, false)
	if err != nil {
		return "", fnerrors.RemoteError("error loading image: %w", err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	response := string(b)
	if err != nil {
		return response, fnerrors.RemoteError("error reading load response body: %w", err)
	}
	return response, nil
}
