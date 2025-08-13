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
	"os"
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/cenkalti/backoff"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/storage"
	"namespacelabs.dev/integrations/auth"
)

const (
	mainArtifactNamespace = "main"
	cacheURLRetries       = 3
)

func NewArtifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Artifact-related activities.",
	}

	cmd.AddCommand(newArtifactUploadCmd())
	cmd.AddCommand(newArtifactDownloadCmd())
	cmd.AddCommand(newArtifactCacheURLCmd())

	return cmd
}

func newArtifactUploadCmd() *cobra.Command {
	var namespace string
	var expirationDur time.Duration

	return fncobra.Cmd(&cobra.Command{
		Use:   "upload [src] [dest]",
		Short: "Upload an artifact.",
		Long:  "Upload an artifact. Currently, only single file uploads are supported.",
		Args:  cobra.ExactArgs(2),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&namespace, "namespace", mainArtifactNamespace, "Target namespace of the artifact.")
		flags.DurationVar(&expirationDur, "expires_in", 0, "If set, sets the artifact's expiration into the specified future.")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		if len(args) != 2 {
			return fnerrors.Newf("expected exactly two arguments: a local source and a remote destination")
		}
		src, dest := args[0], args[1]

		uploadFile, err := os.Open(src)
		if err != nil {
			return fnerrors.Newf("failed to open file %s: %w", src, err)
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

		opts := storage.UploadOpts{}

		if expirationDur > 0 {
			opts.ExpiresAt = ptr.Time(time.Now().Add(expirationDur))
		} else if expirationDur < 0 {
			return fnerrors.BadInputError("expiration can't be negative")
		}

		if _, err := storage.UploadArtifactWithOpts(ctx, cli, namespace, dest, uploadFile, opts); err != nil {
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
			return fnerrors.Newf("expected exactly two arguments: a remote source and a local destination")
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
		flags.StringVar(&dest, "out", "", "Filename to save the downloaded content at.")
		cobra.MarkFlagRequired(flags, "out")
		maxAgeFlag = flags.Lookup("max_age")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		now := time.Now()
		sourceURL := args[0]

		token, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.BadDataError("failed to obtain auth data: %w", err)
		}

		cli, err := storage.NewClient(ctx, token)
		if err != nil {
			return fnerrors.InvocationError("namespace api", "failed to connect: %w", err)
		}
		defer cli.Close()

		var newerThan time.Time
		if maxAgeFlag.Changed {
			newerThan = now.Add(-maxAge)
		}

		return backoff.RetryNotify(func() error {
			r, _, err := storage.CacheURL(ctx, cli, sourceURL, storage.CacheURLOpts{
				NewerThan: newerThan,
				Logf: func(f string, xs ...interface{}) {
					fmt.Fprintf(console.Stderr(ctx), f, xs...)
				},
			})

			if err != nil {
				if status.Code(err) == codes.InvalidArgument {
					return backoff.Permanent(fnerrors.BadInputError("%w", err))
				}

				if cse := new(storage.CacheSourceError); errors.As(err, cse) {
					if cse.HTTPStatusCode == 0 || cse.HTTPStatusCode >= http.StatusInternalServerError {
						// Only retry network errors or 5xx HTTP errors.
						return err
					}
				}

				return backoff.Permanent(fnerrors.InvocationError("remote", "%w", err))
			}

			w, err := os.Create(dest)
			if err != nil {
				return backoff.Permanent(fnerrors.Newf("failed to open %q: %w", dest, err))
			}
			defer w.Close()

			if _, err := w.ReadFrom(r); err != nil {
				return backoff.Permanent(fnerrors.Newf("failed to download artifact: %w", err))
			}

			fmt.Fprintf(console.Stderr(ctx), "Downloaded to %s.\n", dest)
			return nil
		},
			backoff.WithMaxRetries(backoff.WithContext(backoff.NewConstantBackOff(5*time.Second), ctx), cacheURLRetries),
			func(err error, delay time.Duration) {
				fmt.Fprintf(console.Stderr(ctx), "Error: Failed to cache artifact: %v; retrying in %v...\n", err, delay)
			})
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
		return fnerrors.Newf("failed to open %q: %w", dest, err)
	}
	defer w.Close()

	if _, err := io.Copy(w, reader); err != nil {
		return fnerrors.Newf("failed to write %q: %w", dest, err)
	}
	return nil
}
