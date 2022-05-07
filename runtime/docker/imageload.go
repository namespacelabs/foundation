// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/ctxio"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/compute"
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
		return checkWriteImage(ctx, img, config, ref, ensureTag)
	})
}

func checkWriteImage(ctx context.Context, img v1.Image, config v1.Hash, ref name.Tag, ensureTag bool) error {
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
		return fnerrors.InvocationError("failed to push to docker: %w", err)
	}

	tasks.Attachments(ctx).AddResult("uploaded", true)
	return nil
}

// Write saves the image into the daemon as the given tag.
func writeImage(ctx context.Context, client Client, tag name.Tag, img v1.Image) (string, error) {
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(tarball.Write(tag, img, ctxio.WriterWithContext(ctx, pw, nil)))
	}()

	progressReader := artifacts.NewProgressReader(pr, 0)
	tasks.Attachments(ctx).SetProgress(progressReader)

	// write the image in docker save format first, then load it
	resp, err := client.ImageLoad(ctx, progressReader, false)
	if err != nil {
		return "", fnerrors.InvocationError("error loading image: %w", err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	response := string(b)
	if err != nil {
		return response, fnerrors.InvocationError("error reading load response body: %w", err)
	}
	return response, nil
}

func writeImageOnce(name string, img v1.Image) (compute.Computable[v1.Hash], error) {
	digest, err := img.Digest()
	if err != nil {
		return nil, fnerrors.InvocationError("docker: failed to fetch compute image digest: %w", err)
	}

	config, err := img.ConfigName()
	if err != nil {
		return nil, fnerrors.InvocationError("docker: failed to fetch image config: %w", err)
	}

	if name == "" {
		name = "foundation.namespacelabs.dev/docker-invocation"
	}

	return &writeImageOnceImpl{imageName: name, image: img, digest: digest, config: config}, nil
}

type writeImageOnceImpl struct {
	imageName      string
	image          v1.Image
	digest, config v1.Hash

	compute.DoScoped[v1.Hash]
}

var _ compute.Computable[v1.Hash] = &writeImageOnceImpl{}

func (w *writeImageOnceImpl) Action() *tasks.ActionEvent {
	return tasks.Action("docker.image-load").Arg("ref", w.imageName).Arg("digest", w.digest).Arg("config", w.config)
}

func (w *writeImageOnceImpl) Inputs() *compute.In {
	return compute.Inputs().Str("imageName", w.imageName).JSON("digest", w.digest)
}

func (w *writeImageOnceImpl) Compute(ctx context.Context, _ compute.Resolved) (v1.Hash, error) {
	tag, err := name.NewTag(w.imageName, name.WithDefaultTag("local"))
	if err != nil {
		return v1.Hash{}, err
	}

	if err := checkWriteImage(ctx, w.image, w.config, tag, false); err != nil {
		return v1.Hash{}, err
	}

	return w.config, nil
}
