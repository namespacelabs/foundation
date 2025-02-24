// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	storagev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/storage/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/cenkalti/backoff"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/storage"
	"namespacelabs.dev/integrations/auth"
)

const (
	mainArtifactNamespace  = "main"
	cacheArtifactNamespace = "cache"
	cacheSourceURLLabel    = "cache.source-url"
	cacheURLRetries        = 3
)

var (
	// Matches server-side validation regexp.
	validPath = regexp.MustCompile("[a-zA-Z0-9][a-zA-Z0-9-_./]*[a-zA-Z0-9]")
	slashRuns = regexp.MustCompile("/+")
)

func NewArtifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "artifact",
		Short:  "Artifact-related activities.",
		Hidden: true,
	}

	cmd.AddCommand(newArtifactUploadCmd())
	cmd.AddCommand(newArtifactDownloadCmd())
	cmd.AddCommand(newArtifactCacheURLCmd())

	return cmd
}

func newArtifactUploadCmd() *cobra.Command {
	var namespace string

	return fncobra.Cmd(&cobra.Command{
		Use:   "upload [src] [dest]",
		Short: "Upload an artifact.",
		Long:  "Upload an artifact. Currently, only single file uploads are supported.",
		Args:  cobra.ExactArgs(2),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&namespace, "namespace", mainArtifactNamespace, "Target namespace of the artifact.")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		if len(args) != 2 {
			return fnerrors.New("expected exactly two arguments: a local source and a remote destination")
		}
		src, dest := args[0], args[1]

		uploadFile, err := os.Open(src)
		if err != nil {
			return fnerrors.New("failed to open file %s: %w", src, err)
		}
		defer uploadFile.Close()

		token, err := auth.LoadDefaults()
		if err != nil {
			return err
		}

		cli, err := storage.NewClient(ctx, token)
		if err != nil {
			return err
		}
		defer cli.Close()

		if err := storage.UploadArtifact(ctx, cli, namespace, dest, uploadFile); err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "Uploaded %s to %s (namespace %s)\n", src, dest, namespace)

		return nil
	})
}

func newArtifactDownloadCmd() *cobra.Command {
	var namespace string

	return fncobra.Cmd(&cobra.Command{
		Use:   "download [src] [dest]",
		Short: "Download an artifact.",
		Long:  "Download an artifact. Currently, only single file downloads are supported.",
		Args:  cobra.ExactArgs(2),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&namespace, "namespace", mainArtifactNamespace, "Namespace of the artifact.")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		if len(args) != 2 {
			return fnerrors.New("expected exactly two arguments: a remote source and a local destination")
		}
		src, dest := args[0], args[1]

		token, err := auth.LoadDefaults()
		if err != nil {
			return err
		}

		cli, err := storage.NewClient(ctx, token)
		if err != nil {
			return err
		}
		defer cli.Close()

		if err := writeArtifact(ctx, cli, namespace, src, dest); err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "Downloaded %s (namespace %s) to %s\n", src, namespace, dest)

		return nil
	})
}

func newArtifactCacheURLCmd() *cobra.Command {
	var dest string
	var maxAge time.Duration
	var maxAgeFlag *pflag.Flag

	return fncobra.Cmd(&cobra.Command{
		Use:   "cache-url [target url] --out=[filename] { --max_age=[duration] }",
		Short: "Download an arbitrary URL using a pull-through cache.",
		Long: `Download the content from an arbitrary URL and cache it for fast access.

If the content is already present in the artifact cache for the given URL it will be used instead.

The content at the URL is assumed to be immutable.`,
		Args: cobra.ExactArgs(1),
	}).WithFlags(func(flags *pflag.FlagSet) {
		//flags.BoolVar(&renew, "renew", false, "Force-download from the source and update the cached content.")
		flags.DurationVar(&maxAge, "max_age", 0, "Redownload from source if the cached content is older than this duration.")
		flags.StringVar(&dest, "destination", "", "Filename to save the downloaded content at.")
		cobra.MarkFlagRequired(flags, "destination")
		maxAgeFlag = flags.Lookup("max_age")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		now := time.Now()
		sourceURL := args[0]

		parsedURL, err := url.Parse(sourceURL)
		if err != nil {
			return fnerrors.BadInputError("invalid URL format: %w", err)
		}

		token, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.BadDataError("failed to obtain auth data: %w", err)
		}

		cli, err := storage.NewClient(ctx, token)
		if err != nil {
			return fnerrors.InvocationError("namespace api", "failed to connect: %w", err)
		}
		defer cli.Close()

		listResp, err := cli.Artifacts.ListArtifacts(ctx, &storagev1beta.ListArtifactsRequest{
			Namespaces:  []string{cacheArtifactNamespace},
			LabelFilter: []*stdlib.LabelFilterEntry{{Name: cacheSourceURLLabel, Value: sourceURL, Op: stdlib.LabelFilterEntry_EQUAL}},
		})
		if err != nil {
			return fnerrors.InvocationError("namespace api", "failed to list artifacts: %w", err)
		}

		var newest *storagev1beta.Artifact
		for _, art := range listResp.Artifacts {
			if newest == nil || art.GetCreatedAt().AsTime().After(newest.GetCreatedAt().AsTime()) {
				newest = art
			}
		}

		if newest != nil {
			if !maxAgeFlag.Changed || now.Sub(newest.GetCreatedAt().AsTime()) <= maxAge {
				fmt.Fprintf(console.Stderr(ctx), "Downloading from cache (cached at %v)...\n", newest.GetCreatedAt().AsTime())
				if err := writeArtifact(ctx, cli, newest.GetNamespace(), newest.GetPath(), dest); err != nil {
					return err
				}
				fmt.Fprintf(console.Stderr(ctx), "Downloaded to %s.\n", dest)
				return nil
			} else {
				fmt.Fprintf(console.Stderr(ctx), "Content cached at %v is too old, downloading from source...\n", newest.GetCreatedAt().AsTime())
				// Fallthrough
			}
		} else {
			fmt.Fprintf(console.Stderr(ctx), "Artifact not found in cache; downloading from source...\n")
			// Fallthough
		}

		cachePath := cacheArtifactPath(now, parsedURL)
		labels := map[string]string{cacheSourceURLLabel: sourceURL}

		err = backoff.RetryNotify(func() error {
			req, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
			if err != nil {
				return backoff.Permanent(fnerrors.InvocationError("remote url", "failed to prepare request: %w", err))
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fnerrors.InvocationError("remote url", "failed to send request: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode >= http.StatusInternalServerError {
				return fnerrors.InvocationError("remote url", "remote server returned status code %d", resp.StatusCode)
			} else if resp.StatusCode != http.StatusOK { // treat non-server error as permanent
				return backoff.Permanent(fnerrors.InvocationError("remote url", "remote server returned status code %d", resp.StatusCode))
			}

			w, err := os.Create(dest)
			if err != nil {
				return backoff.Permanent(fnerrors.New("failed to open %q: %w", dest, err))
			}
			defer w.Close()

			// Uploading the response will write it to the file as a side-effect.
			// Read errors will be tagged to make them retriable below.
			teeR := io.TeeReader(wrapErrorsReader{resp.Body}, w)

			var length int64
			if resp.ContentLength >= 0 {
				length = resp.ContentLength
			}

			if err := storage.UploadArtifactWithOpts(ctx, cli, cacheArtifactNamespace, cachePath, teeR, storage.UploadOpts{
				Labels: labels,
				Length: length,
			}); err != nil {
				if rre := new(remoteReadError); errors.As(err, rre) {
					return rre.Unwrap()
				} else {
					return backoff.Permanent(fnerrors.InvocationError("remote url", "failed to cache artifact: %w", err))
				}
			}

			return nil
		},
			backoff.WithMaxRetries(backoff.WithContext(backoff.NewConstantBackOff(5*time.Second), ctx), cacheURLRetries),
			func(err error, delay time.Duration) {
				fmt.Fprintf(console.Stderr(ctx), "Error: Failed to cache artifact: %v; retrying in %v...\n", err, delay)
			})
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Downloaded to %s.\n", dest)
		return nil
	})
}

func writeArtifact(ctx context.Context, cli storage.Client, namespace, path string, dest string) error {
	reader, err := storage.ResolveArtifactStream(ctx, cli, namespace, path)
	if err != nil {
		return err
	}
	defer reader.Close()

	w, err := os.Create(dest)
	if err != nil {
		return fnerrors.New("failed to open %q: %w", dest, err)
	}
	defer w.Close()

	if _, err := io.Copy(w, reader); err != nil {
		return fnerrors.New("failed to write %q: %w", dest, err)
	}
	return nil
}

func cacheArtifactPath(now time.Time, sourceURL *url.URL) string {
	safePath := slashRuns.ReplaceAllString(strings.Join(validPath.FindAllString(sourceURL.Path, -1), "-"), "/")

	p := now.Format("2006-01-02_15.04.05")
	p += "/" + sourceURL.Hostname()
	if safePath != "" {
		p += "/" + safePath
	}

	return p
}

type wrapErrorsReader struct {
	io.Reader
}

func (r wrapErrorsReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err != nil {
		return n, remoteReadError{err}
	}
	return
}

type remoteReadError struct {
	err error
}

func (err remoteReadError) Error() string {
	return err.err.Error()
}

func (err remoteReadError) Unwrap() error {
	return err.err
}
