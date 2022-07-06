// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
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
	ctx := context.Background()

	fncobra.SetupViper()
	compute.RegisterProtoCacheable()
	compute.RegisterBytesCacheable()
	fscache.RegisterFSCacheable()

	sink, style, cleanup := fncobra.ConsoleToSink(fncobra.StandardConsole())
	ctxWithSink := colors.WithStyle(tasks.WithSink(ctx, sink), style)

	var path string
	err := compute.Do(ctxWithSink, func(ctx context.Context) (err error) {
		path, err = run(ctx)
		return
	})
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		fnerrors.Format(os.Stderr, err, fnerrors.WithStyle(style))
		return
	}

	// We make sure to flush all the output before starting the command.

	proc := exec.CommandContext(ctx, filepath.Join(path, "ns"), os.Args[1:]...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	err = proc.Run()
	if err != nil {
		fnerrors.Format(os.Stderr, err, fnerrors.WithStyle(style))
	}
}

func run(ctx context.Context) (path string, err error) {
	latestVersion, needUpdate, err := getCachedVersion(ctx)
	if err != nil {
		return "", fnerrors.Wrapf(nil, err, "failed to load versions.json")
	}

	if latestVersion == nil || needUpdate {
		path, err = performUpdate(ctx)
	} else {
		path, err = fetchBinary(ctx, latestVersion)
	}
	return
}

// Returns the latest version of the tool.
// If the current version cache is stale it also hits the remote endpoint.
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

func performUpdate(ctx context.Context) (string, error) {
	newVersion, err := loadRemoteVersion(ctx)
	if err != nil {
		return "", fnerrors.Wrapf(nil, err, "failed to load an update from the update service")
	}
	path, err := fetchBinary(ctx, newVersion)
	if err != nil {
		return "", fnerrors.Wrapf(nil, err, "failed to fetch a new tarball")
	}
	if err := persistCache(newVersion); err != nil {
		return "", fnerrors.Wrapf(nil, err, "failed to persist the version cache")
	}
	return path, nil
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
	bs, err := ioutil.ReadFile(cachePath)
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
	if err := ioutil.WriteFile(cachePath, bs, 0644); err != nil {
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

	fsys := unpack.Unpack("tool", tarfs.TarGunzip(download.URL(tarRef)))
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
