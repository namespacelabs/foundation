// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	updatePeriod = time.Hour * 24
)

// Represents a tool version reference as reported by the version-check endpoint.
type toolVersion struct {
	TagName   string    `json:"tag_name"`
	BuildTime time.Time `json:"build_time"`
	FetchedAt time.Time `json:"fetched_at"`
	URL       string    `json:"tarball_url"`
	SHA256    string    `json:"tarball_sha256"`
}

// Schema for $CACHE/tool/versions.json.
type versionCache struct {
	Latest *toolVersion `json:"latest"`
}

func main() {
	fncobra.SetupViper()
	compute.RegisterProtoCacheable()
	compute.RegisterBytesCacheable()
	fscache.RegisterFSCacheable()

	sink, style, cleanup := fncobra.ConsoleToSink(fncobra.StandardConsole())
	ctxWithSink := colors.WithStyle(tasks.WithSink(context.Background(), sink), style)

	// It's a bit awkward, but the main command execution is split between the command proper
	// and the execution of the inner ns binary after all the nsboot cleanup is done.
	// This variable is passes the path to execute from inside the command to the outside.
	var pathToExec string

	rootCmd := &cobra.Command{
		Use:                "nsboot",
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,

		RunE: func(cmd *cobra.Command, args []string) (err error) {
			pathToExec, err = updateAndRun(cmd.Context())
			return
		},
	}
	rootCmd.AddCommand(&cobra.Command{
		Use:   "update-ns",
		Short: "Checks and downloads updates for the ns command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return forceUpdate(cmd.Context())
		},
	})
	rootCmd.Flags().ParseErrorsWhitelist.UnknownFlags = true
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	err := compute.Do(ctxWithSink, func(ctx context.Context) (err error) {
		return rootCmd.ExecuteContext(ctx)
	})
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		format.Format(os.Stderr, err, format.WithStyle(style))
		return
	}

	// We make sure to flush all the output before starting the command.
	if pathToExec != "" {
		proc := exec.CommandContext(context.Background(), filepath.Join(pathToExec, "ns"), os.Args[1:]...)
		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		proc.Env = append(os.Environ(), fmt.Sprintf("NSBOOT_VERSION=%s", formatNSBootVersion()))
		err = proc.Run()
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				os.Exit(exiterr.ExitCode())
			} else {
				format.Format(os.Stderr, err, format.WithStyle(style))
				os.Exit(3)
			}
		}
	}
}

// Performs the normal logic of checking for updates lazily.
func updateAndRun(ctx context.Context) (path string, err error) {
	latestVersion, needUpdate, err := getCachedVersion(ctx)
	if err != nil {
		return "", fnerrors.Wrapf(nil, err, "failed to load versions.json")
	}

	if latestVersion == nil || needUpdate {
		_, path, err = performUpdate(ctx)
	} else {
		path, err = fetchBinary(ctx, latestVersion)
	}
	return
}

func forceUpdate(ctx context.Context) error {
	cachedVersion, _, err := getCachedVersion(ctx)
	if err != nil {
		return err
	}
	newVersion, _, err := performUpdate(ctx)
	if err != nil {
		return err
	}
	if cachedVersion.TagName == newVersion.TagName {
		fmt.Fprintf(console.Stdout(ctx), "Already up-to-date at %s.\n", newVersion.TagName)
	} else {
		fmt.Fprintf(console.Stdout(ctx), "Updated to version %s.\n", newVersion.TagName)
	}

	return nil
}

// Loads version cache and applies default update policy.
func getCachedVersion(ctx context.Context) (version *toolVersion, needUpdate bool, err error) {
	cache, err := loadVersionCache()
	if err != nil {
		return nil, false, err
	}
	if cache == nil {
		return nil, false, err
	}
	enableAutoupdate := viper.GetBool("enable_autoupdate")
	version = cache.Latest
	stale := cache.Latest != nil && time.Since(cache.Latest.FetchedAt) > updatePeriod
	needUpdate = stale && enableAutoupdate
	if stale && !enableAutoupdate {
		fmt.Fprintf(console.Stdout(ctx), "ns version is stale, but auto-update is disabled (see \"enable_autoupdate\" in config.json)\n")
	}
	return
}

func performUpdate(ctx context.Context) (*toolVersion, string, error) {
	newVersion, err := loadRemoteVersion(ctx)
	if err != nil {
		return nil, "", fnerrors.Wrapf(nil, err, "failed to load an update from the update service")
	}
	path, err := fetchBinary(ctx, newVersion)
	if err != nil {
		return nil, "", fnerrors.Wrapf(nil, err, "failed to fetch a new tarball")
	}
	if err := persistCache(newVersion); err != nil {
		return nil, "", fnerrors.Wrapf(nil, err, "failed to persist the version cache")
	}
	return newVersion, path, nil
}

func loadRemoteVersion(ctx context.Context) (*toolVersion, error) {
	return tasks.Return(ctx, tasks.Action("version-check"), func(ctx context.Context) (*toolVersion, error) {
		resp, err := fnapi.GetLatestVersion(ctx, nil)
		if err != nil {
			return nil, err
		}

		tarball := findHostTarball(resp.Tarballs)
		if tarball == nil {
			return nil, fnerrors.New("no tarball for host OS/architecture offered by the update service")
		}

		return &toolVersion{
			TagName:   resp.Version,
			BuildTime: resp.BuildTime,
			FetchedAt: time.Now(),
			URL:       tarball.URL,
			SHA256:    tarball.SHA256,
		}, nil
	})
}

func loadVersionCache() (*versionCache, error) {
	cachePath, err := cachePath()
	if err != nil {
		return nil, err
	}
	bs, err := os.ReadFile(cachePath)
	if os.IsNotExist(err) {
		// Missing the cache is okay.
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var cache versionCache
	err = json.Unmarshal(bs, &cache)
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

func persistCache(version *toolVersion) error {
	// Reload to reduce the effects of concurrent writes.
	cache, err := loadVersionCache()
	if err != nil {
		return err
	}
	if cache == nil {
		cache = &versionCache{}
	}
	cache.Latest = version
	bs, err := json.MarshalIndent(cache, "", "\t")
	if err != nil {
		return err
	}
	cachePath, err := cachePath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(cachePath, bs, 0644); err != nil {
		return err
	}
	return nil
}

func cachePath() (string, error) {
	toolDir, err := dirs.Ensure(dirs.Cache())
	if err != nil {
		return "", err
	}
	return filepath.Join(toolDir, "versions.json"), nil
}

func fetchBinary(ctx context.Context, version *toolVersion) (string, error) {
	tarRef := artifacts.Reference{
		URL: version.URL,
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       version.SHA256,
		},
	}

	fsys := unpack.Unpack("tool", tarfs.TarGunzip(download.NamespaceURL(tarRef)), unpack.SkipChecksumCheck())
	unpacked, err := compute.GetValue(ctx, fsys)
	if err != nil {
		return "", err
	}
	return unpacked.Files, nil
}

func findHostTarball(tarballs []*fnapi.Artifact) *fnapi.Artifact {
	for _, tarball := range tarballs {
		goos := strings.ToLower(tarball.OS)
		goarch := strings.ToLower(tarball.Arch)
		if goos == runtime.GOOS && goarch == runtime.GOARCH {
			return tarball
		}
	}
	return nil
}

func formatNSBootVersion() string {
	ver, err := version.Current()
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err)
	}
	bs, err := protojson.Marshal(ver)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err)
	}
	return string(bs)
}
