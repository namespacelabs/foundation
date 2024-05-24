// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nsboot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/cli/versioncheck"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	updatePeriod = time.Hour * 24
)

type NSPackage struct {
	Command string
	Path    string
}

func versionsJson(command string) string {
	return fmt.Sprintf("%s-version.json", command)
}

func (p NSPackage) Execute(ctx context.Context) error {
	proc := exec.CommandContext(ctx, filepath.Join(string(p.Path), p.Command), os.Args[1:]...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Env = append(os.Environ(), fmt.Sprintf("NSBOOT_VERSION=%s", formatNSBootVersion()))
	return proc.Run()
}

func (p NSPackage) ExecuteAndForwardExitCode(ctx context.Context, style colors.Style) {
	err := p.Execute(ctx)
	if err == nil {
		os.Exit(0)
	} else if exiterr, ok := err.(*exec.ExitError); ok {
		os.Exit(exiterr.ExitCode())
	} else {
		format.Format(os.Stderr, err, format.WithStyle(style))
		os.Exit(3)
	}
}

func checkExists(command string, cached *versionCache) (NSPackage, bool) {
	if cached != nil && cached.BinaryPath != "" {
		if _, err := os.Stat(cached.BinaryPath); err == nil {
			return NSPackage{Command: command, Path: cached.BinaryPath}, true
		}
	}

	return NSPackage{}, false
}

func UpdateIfNeeded(ctx context.Context, command string, currentVersion string) (*toolVersion, NSPackage, error) {
	return checkAndDoUpdate(ctx, command, currentVersion, true)
}

func CheckCachedUpdate(ctx context.Context, command string, currentVersion string) (*toolVersion, NSPackage, error) {
	return checkAndDoUpdate(ctx, command, currentVersion, false)
}

func checkAndDoUpdate(ctx context.Context, command string, currentVersion string, doUpdate bool) (*toolVersion, NSPackage, error) {
	cached, needUpdate, err := loadCheckCachedMetadata(ctx, command, true)
	if err != nil {
		return nil, NSPackage{}, fnerrors.New("failed to load versions.json: %w", err)
	}

	if cached != nil {
		if cached.Latest != nil && currentVersion != "" && semver.Compare(currentVersion, cached.Latest.TagName) >= 0 {
			// Current version is at least as up to date as the downloaded version.
			return nil, NSPackage{}, nil
		}

		if !needUpdate {
			if ns, ok := checkExists(command, cached); ok {
				return cached.Latest, ns, nil
			}
		}
	}

	if !doUpdate {
		return nil, NSPackage{}, nil
	}

	var ver *toolVersion
	var ns NSPackage
	ns.Command = command

	updateErr := compute.Do(ctx, func(ctx context.Context) error {
		var err error
		ver, ns, err = performUpdate(ctx, command, cached, currentVersion, !needUpdate)
		return err
	})

	return ver, ns, updateErr
}

func ForceUpdate(ctx context.Context, command string) error {
	cached, err := loadCachedMetadata(command)
	if err != nil {
		return err
	}

	newVersion, _, err := performUpdate(ctx, command, cached, "", true)
	if err != nil {
		return err
	}

	if cached != nil && cached.Latest != nil && cached.Latest.TagName == newVersion.TagName {
		fmt.Fprintf(console.Stdout(ctx), "Already up-to-date at %s.\n", newVersion.TagName)
	} else {
		fmt.Fprintf(console.Stdout(ctx), "Updated to version %s.\n", newVersion.TagName)
	}

	return nil
}

func AutoUpdateEnabled() bool {
	return viper.GetBool("enable_autoupdate")
}

// Loads version cache and applies default update policy.
func loadCheckCachedMetadata(ctx context.Context, command string, silent bool) (*versionCache, bool, error) {
	cache, err := loadCachedMetadata(command)
	if err != nil {
		return nil, false, err
	}

	if cache == nil {
		return nil, false, err
	}

	needUpdate := cache.Latest != nil && (time.Since(cache.Latest.FetchedAt) > updatePeriod || cache.PendingVersion != "")
	if needUpdate && !AutoUpdateEnabled() {
		needUpdate = false
		if !silent {
			fmt.Fprintf(console.Stdout(ctx), "ns version is stale, but auto-update is disabled (see \"enable_autoupdate\" in config.json)\n")
		}
	}

	return cache, needUpdate, nil
}

func performUpdate(ctx context.Context, command string, previous *versionCache, currentVersion string, forceUpdate bool) (*toolVersion, NSPackage, error) {
	// XXX store the last time timestamp checked, else after 24h we're always checking.
	newVersion, err := fetchRemoteVersion(ctx, command)
	if err != nil {
		return nil, NSPackage{}, fnerrors.New("failed to load an update from the update service: %w", err)
	}

	// If the latest version is not newer than the current, just use the current binary.
	if currentVersion != "" && semver.Compare(currentVersion, newVersion.TagName) >= 0 {
		return nil, NSPackage{}, nil
	}

	values := url.Values{}

	if previous != nil && previous.Latest != nil {
		serialized, _ := json.Marshal(reportedExistingVersion{
			TagName: previous.Latest.TagName,
			SHA256:  previous.Latest.SHA256,
		})

		values.Add("update_from", base64.RawURLEncoding.EncodeToString(serialized))
	}

	if forceUpdate {
		values.Add("force_update", "true")
	}

	path, err := fetchBinary(ctx, command, newVersion, values)
	if err != nil {
		return nil, NSPackage{}, fnerrors.New("failed to fetch a new tarball: %w", err)
	}

	// Only commit to the new version once we know that we got the new binary.
	if err := persistVersion(command, newVersion, path); err != nil {
		return nil, NSPackage{}, fnerrors.New("failed to persist the version cache: %w", err)
	}

	return newVersion, NSPackage{Command: command, Path: path}, nil
}

func fetchRemoteVersion(ctx context.Context, command string) (*toolVersion, error) {
	return tasks.Return(ctx, tasks.Action(command+".version-check"), func(ctx context.Context) (*toolVersion, error) {
		resp, err := fnapi.GetLatestVersion(ctx, map[string]any{command: map[string]any{}})
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

func loadCachedMetadata(command string) (*versionCache, error) {
	cacheDir, err := dirs.Cache()
	if err != nil {
		return nil, err
	}

	bs, err := os.ReadFile(filepath.Join(cacheDir, versionsJson(command)))
	if err != nil {
		if os.IsNotExist(err) {
			// Missing the cache is okay.
			return nil, nil
		}

		return nil, err
	}

	var cache versionCache
	if err := json.Unmarshal(bs, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func NewerVersion(command string, ver *versioncheck.Status) bool {
	cached, err := loadCachedMetadata(command)
	if err == nil && cached != nil && cached.Latest != nil {
		return semver.Compare(ver.Version, cached.Latest.TagName) > 0
	}

	return false
}

func invalidate(command string, cached *versionCache, pending string) error {
	dir, err := dirs.Cache()
	if err != nil {
		return err
	}

	return rewrite(dir, versionsJson(command), func(w io.Writer) error {
		copy := *cached
		copy.PendingVersion = pending
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(copy)
	})
}

func persistVersion(command string, version *toolVersion, path string) error {
	toolDir, err := dirs.Ensure(dirs.Cache())
	if err != nil {
		return err
	}

	return rewrite(toolDir, versionsJson(command), func(w io.Writer) error {
		cache := versionCache{
			Latest:     version,
			BinaryPath: path,
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(cache)
	})
}

func rewrite(dir, filename string, make func(io.Writer) error) error {
	newFile, err := os.CreateTemp(dir, filename)
	if err != nil {
		return fnerrors.InternalError("failed to create new %s: %w", filename, err)
	}

	writeErr := make(newFile)
	// Always close before returning.
	closeErr := newFile.Close()

	if writeErr != nil {
		return fnerrors.InternalError("failed to generate new %s: %w", filename, err)
	}

	if closeErr != nil {
		return fnerrors.InternalError("failed to flush new %s: %w", filename, err)
	}

	if err := os.Rename(newFile.Name(), filepath.Join(dir, filename)); err != nil {
		return fnerrors.InternalError("failed to update %s: %w", filename, err)
	}

	return nil
}

func fetchBinary(ctx context.Context, command string, version *toolVersion, values url.Values) (string, error) {
	tarRef := artifacts.Reference{
		URL: version.URL,
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       version.SHA256,
		},
	}

	fsys := unpack.Unpack(command, tarfs.TarGunzip(download.NamespaceURL(tarRef, values)), unpack.SkipChecksumCheck())
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
