// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"archive/zip"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	storagev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/storage/v1beta"
	"github.com/aws/smithy-go/ptr"
	"github.com/cenkalti/backoff"
	"github.com/dustin/go-humanize"
	"github.com/mattn/go-zglob"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/framework/io/downloader"
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
	cmd.AddCommand(newArtifactExpireCmd())

	return cmd
}

func newArtifactUploadCmd() *cobra.Command {
	var namespace string
	var expirationDur time.Duration
	var pack string

	return fncobra.Cmd(&cobra.Command{
		Use:   "upload [src] [dest]",
		Short: "Upload an artifact.",
		Long:  "Upload an artifact. Currently, only single file uploads are supported.",
		Args:  cobra.RangeArgs(1, 2),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&namespace, "namespace", mainArtifactNamespace, "Target namespace of the artifact.")
		flags.DurationVar(&expirationDur, "expires_in", 0, "If set, sets the artifact's expiration into the specified future.")
		flags.StringVar(&pack, "pack", "", "A glob pattern to select files to zip and upload.")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		var src, dest string
		var uploadFile io.ReadSeekCloser

		if pack != "" {
			if len(args) != 1 {
				return fnerrors.Newf("expected exactly one argument (destination) when --pack is provided")
			}
			dest = args[0]

			tmpFile, err := os.CreateTemp("", "artifact-upload-*.zip")
			if err != nil {
				return fnerrors.Newf("failed to create temporary file: %w", err)
			}
			defer os.Remove(tmpFile.Name())

			start := time.Now()
			count, err := zipFiles(ctx, pack, tmpFile)
			if err != nil {
				tmpFile.Close()
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "Created archive with %d entries. Took %v.\n", count, time.Since(start))

			if _, err := tmpFile.Seek(0, 0); err != nil {
				tmpFile.Close()
				return err
			}

			uploadFile = tmpFile
			src = "archive"
		} else {
			if len(args) != 2 {
				return fnerrors.Newf("expected exactly two arguments: a local source and a remote destination")
			}
			src, dest = args[0], args[1]

			f, err := os.Open(src)
			if err != nil {
				return fnerrors.Newf("failed to open file %s: %w", src, err)
			}
			uploadFile = f
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

		start := time.Now()
		if _, err := storage.UploadArtifactWithOpts(ctx, cli, namespace, dest, uploadFile, opts); err != nil {
			return err
		}

		stat, err := uploadFile.Seek(0, 1)
		if err != nil {
			// ignore, just don't print the size
			stat = 0
		}

		fmt.Fprintf(console.Stdout(ctx), "Uploaded %s (%s) to %s (namespace %s). Took %v.\n",
			src, humanize.Bytes(uint64(stat)), dest, namespace, time.Since(start))

		return nil
	})
}

func zipFiles(ctx context.Context, pattern string, w io.Writer) (int, error) {
	matches, err := zglob.Glob(pattern)
	if err != nil {
		return 0, fnerrors.Newf("failed to glob files: %w", err)
	}

	if len(matches) == 0 {
		return 0, fnerrors.Newf("no files matched pattern %q", pattern)
	}

	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestSpeed)
	})
	defer zw.Close()

	count := 0
	seen := map[string]bool{}

	addFile := func(match string, info os.FileInfo) error {
		if seen[match] {
			return nil
		}
		seen[match] = true

		f, err := os.Open(match)
		if err != nil {
			return fnerrors.Newf("failed to open file %q: %w", match, err)
		}
		defer f.Close()

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fnerrors.Newf("failed to create zip header for %q: %w", match, err)
		}
		header.Name = match
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return fnerrors.Newf("failed to create zip writer for %q: %w", match, err)
		}

		if _, err := io.Copy(writer, f); err != nil {
			return fnerrors.Newf("failed to copy file %q to zip: %w", match, err)
		}
		count++
		return nil
	}

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return 0, fnerrors.Newf("failed to stat file %q: %w", match, err)
		}
		if info.IsDir() {
			if err := filepath.Walk(match, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				return addFile(path, info)
			}); err != nil {
				return 0, fnerrors.Newf("failed to walk directory %q: %w", match, err)
			}
			continue
		}

		if err := addFile(match, info); err != nil {
			return 0, err
		}
	}

	return count, nil
}

func newArtifactExpireCmd() *cobra.Command {
	var namespace string

	return fncobra.Cmd(&cobra.Command{
		Use:   "expire [path]",
		Short: "Expire an artifact.",
		Args:  cobra.ExactArgs(1),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&namespace, "namespace", mainArtifactNamespace, "Namespace of the artifact.")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fnerrors.Newf("expected exactly one arguments: the path of the artifact to expire")
		}
		path := args[0]

		token, err := auth.LoadDefaults()
		if err != nil {
			return err
		}

		cli, err := storage.NewClient(ctx, token)
		if err != nil {
			return err
		}
		defer cli.Close()

		if err := storage.ExpireArtifact(ctx, cli, namespace, path); err != nil {
			return err
		}
		fmt.Fprintf(console.Stdout(ctx), "Expired %s (namespace %s)\n", path, namespace)

		return nil
	})
}

func newArtifactDownloadCmd() *cobra.Command {
	var namespace string
	var resume, unpack bool

	return fncobra.Cmd(&cobra.Command{
		Use:   "download [src] [dest]",
		Short: "Download an artifact.",
		Long:  "Download an artifact. Currently, only single file downloads are supported.",
		Args:  cobra.ExactArgs(2),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&namespace, "namespace", mainArtifactNamespace, "Namespace of the artifact.")
		flags.BoolVar(&resume, "resume", false, "Enable resumable downloads with persistent state file.")
		flags.BoolVar(&unpack, "unpack", false, "Unpack the downloaded artifact (assumed to be a zip) into the destination directory.")
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

		downloadTo := dest
		if unpack {
			tmpFile, err := os.CreateTemp("", "artifact-download-*.zip")
			if err != nil {
				return fnerrors.Newf("failed to create temporary file: %w", err)
			}
			defer os.Remove(tmpFile.Name())
			tmpFile.Close() // downloader will open it
			downloadTo = tmpFile.Name()
		}

		start := time.Now()
		if err := writeArtifactWithResume(ctx, cli, namespace, src, downloadTo, resume); err != nil {
			return err
		}

		stat, err := os.Stat(downloadTo)
		if err != nil {
			return fnerrors.Newf("failed to stat downloaded file: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Downloaded %s (namespace %s) (%s). Took %v.\n",
			src, namespace, humanize.Bytes(uint64(stat.Size())), time.Since(start))

		if unpack {
			start := time.Now()
			if err := unzipArtifact(ctx, downloadTo, dest); err != nil {
				return err
			}
			fmt.Fprintf(console.Stdout(ctx), "Unpacking took %v.\n", time.Since(start))
		} else {
			fmt.Fprintf(console.Stdout(ctx), "Downloaded to %s.\n", dest)
		}

		return nil
	})
}

func unzipArtifact(ctx context.Context, src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fnerrors.Newf("failed to open zip file %q: %w", src, err)
	}
	defer r.Close()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return fnerrors.Newf("failed to create destination directory %q: %w", dest, err)
	}

	absDest, err := filepath.Abs(filepath.Clean(dest))
	if err != nil {
		return fnerrors.Newf("failed to resolve destination path: %w", err)
	}

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Check for Zip Slip
		absPath, err := filepath.Abs(filepath.Clean(fpath))
		if err != nil {
			return fnerrors.Newf("failed to resolve file path: %w", err)
		}

		if !strings.HasPrefix(absPath, absDest+string(os.PathSeparator)) {
			return fnerrors.Newf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return fnerrors.Newf("failed to create directory %q: %w", fpath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return fnerrors.Newf("failed to create directory %q: %w", filepath.Dir(fpath), err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fnerrors.Newf("failed to open output file %q: %w", fpath, err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fnerrors.Newf("failed to open zip file content %q: %w", f.Name, err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return fnerrors.Newf("failed to copy file content to %q: %w", fpath, err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Created %s\n", fpath)
	}

	return nil
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

func writeArtifactWithResume(ctx context.Context, cli storage.Client, namespace, path string, dest string, resume bool) error {
	opts := downloader.Options{
		Resume: resume,
		ResolveURL: func(ctx context.Context) (string, error) {
			res, err := cli.Artifacts.ResolveArtifact(ctx, &storagev1beta.ResolveArtifactRequest{
				Path:      path,
				Namespace: namespace,
			})
			if err != nil {
				return "", err
			}
			return res.SignedDownloadUrl, nil
		},
	}

	return downloader.Download(ctx, dest, opts)
}
