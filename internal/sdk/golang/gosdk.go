// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	//go:embed versions.json
	lib embed.FS

	v     versions
	vonce sync.Once
)

const nixosDynsym = ".dynloader"

type versions struct {
	Versions  map[string]string       `json:"versions"`
	Artifacts map[string]artifactList `json:"artifacts"`
}

type artifactList map[string]string // platform --> digest

func builtin() *versions {
	vonce.Do(func() {
		data, err := fs.ReadFile(lib, "versions.json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &v); err != nil {
			panic(err)
		}
	})
	return &v
}

func MatchSDK(version string, platform specs.Platform) (compute.Computable[LocalSDK], error) {
	v := builtin()

	for ver := range v.Versions {
		if semver.Compare("v"+ver, "v"+version) > 0 {
			version = ver
		}
	}

	return SDK(version, platform)
}

func SDK(version string, platform specs.Platform) (compute.Computable[LocalSDK], error) {
	v := builtin()

	actualVer, has := v.Versions[version]
	if !has {
		return nil, fnerrors.UserError(nil, "go/sdk: no configuration for version %q", version)
	}

	arts, has := v.Artifacts[actualVer]
	if !has {
		return nil, fnerrors.UserError(nil, "go/sdk: no configuration for version %q (was %q)", actualVer, version)
	}

	p := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	digest, has := arts[p]
	if !has {
		return nil, fnerrors.UserError(nil, "go/sdk: no platform configuration for %q in %q", p, actualVer)
	}

	cacheDir, err := dirs.SDKCache("go")
	if err != nil {
		return nil, err
	}

	sdk := LocalSDK{Path: filepath.Join(cacheDir, actualVer), Version: actualVer}
	goBin := filepath.Join(sdk.Path, "go/bin/go")

	if _, err := os.Stat(goBin); err == nil {
		// Go binary already exists.
		if !isNixOS() {
			return compute.Precomputed(sdk, sdk.ComputeDigest), nil
		}

		// Check if dynlink is still valid, if not re-download (so we re-patch).
		st, err := os.Readlink(goBin + nixosDynsym)
		if err == nil {
			if _, err2 := os.Stat(st); err2 == nil {
				// Dynamic loader still exists.
				return compute.Precomputed(sdk, sdk.ComputeDigest), nil
			}
		}
	}

	return &installSDK{
		sdk: sdk,
		tarStream: download.URL(artifacts.Reference{
			URL: fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.tar.gz", actualVer, platform.OS, platform.Architecture),
			Digest: schema.Digest{
				Algorithm: "sha256",
				Hex:       digest,
			},
		}),
	}, nil
}

type LocalSDK struct {
	Path    string
	Version string
}

var _ compute.Digestible = LocalSDK{}

func (sdk LocalSDK) GoRoot() string { return filepath.Join(sdk.Path, "go") }
func (sdk LocalSDK) GoBin() string  { return filepath.Join(sdk.Path, "go/bin/go") }

func (sdk LocalSDK) GoRootEnv() string {
	return fmt.Sprintf("GOROOT=%s", filepath.Join(sdk.Path, "go"))
}

func (sdk LocalSDK) ComputeDigest(context.Context) (schema.Digest, error) {
	return schema.DigestOf(sdk)
}

type installSDK struct {
	sdk       LocalSDK
	tarStream compute.Computable[bytestream.ByteStream]

	compute.DoScoped[LocalSDK]
}

var _ compute.Computable[LocalSDK] = &installSDK{}

func (wfs *installSDK) Action() *tasks.ActionEvent {
	return tasks.Action("go.sdk.install").Arg("path", wfs.sdk.Path).Arg("version", wfs.sdk.Version)
}
func (wfs *installSDK) Inputs() *compute.In {
	return compute.Inputs().Digest("sdk", wfs.sdk).Computable("tarStream", wfs.tarStream)
}
func (wfs *installSDK) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (wfs *installSDK) Compute(ctx context.Context, deps compute.Resolved) (LocalSDK, error) {
	blob := compute.GetDepValue(deps, wfs.tarStream, "tarStream")

	dst := fnfs.ReadWriteLocalFS(wfs.sdk.Path)

	blobFS := tarfs.FS{
		TarStream: func() (io.ReadCloser, error) {
			r, err := blob.Reader()
			if err != nil {
				return nil, err
			}

			pr := artifacts.NewProgressReader(r, blob.ContentLength())
			tasks.Attachments(ctx).SetProgress(pr)

			return gzip.NewReader(pr)
		},
	}

	if err := fnfs.CopyTo(ctx, dst, ".", blobFS); err != nil {
		return LocalSDK{}, err
	}

	if isNixOS() {
		// NixOS does not follow LSB. We need to patch Go's dynamic linker path.
		if err := patchDynloader(ctx, filepath.Join(wfs.sdk.Path, "go/bin/go")); err != nil {
			return LocalSDK{}, err
		}
	}

	return wfs.sdk, nil
}

func isNixOS() bool {
	_, err := os.Stat("/etc/NIXOS")
	return err == nil
}

func patchDynloader(ctx context.Context, binPath string) error {
	return tasks.Action("nixos.patch-elf").Arg("path", binPath).Run(ctx, func(ctx context.Context) error {
		// Assume all NixOS versions have systemd, which is true for all recent versions.
		systemCtl, err := output(ctx, "which", "systemctl")
		if err != nil {
			return err
		}

		existingInterpreter, err := output(ctx, "nix-shell", "-p", "patchelf", "--run", fmt.Sprintf("patchelf --print-interpreter %s", systemCtl))
		if err != nil {
			return err
		}

		cint := strings.TrimSpace(string(existingInterpreter))
		symlink := binPath + nixosDynsym

		if err := os.Remove(symlink); err != nil && os.IsNotExist(err) {
			fmt.Fprintf(console.Stderr(ctx), "Failed to remove %s: %v", symlink, err)
		}

		// Remember which interpreter we used, and if it doesn't exist, re-patch.
		if err := os.Symlink(cint, symlink); err != nil {
			return err
		}

		_, err = output(ctx, "nix-shell", "-p", "patchelf", "--run", fmt.Sprintf("patchelf --set-interpreter %s %s", cint, binPath))
		return err
	})
}

func output(ctx context.Context, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, args[0], args[1:]...)

	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = console.Stderr(ctx)
	c.Stdin = nil
	if err := c.Run(); err != nil {
		return nil, fnerrors.InvocationError("failed to invoke: %w", err)
	}
	return b.Bytes(), nil
}
