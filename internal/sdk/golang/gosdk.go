// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
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
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
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

	ref := artifacts.Reference{
		URL: fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.tar.gz", actualVer, platform.OS, platform.Architecture),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       digest,
		},
	}

	return &prepareSDK{ref: ref, version: actualVer}, nil
}

type prepareSDK struct {
	version string
	ref     artifacts.Reference

	compute.DoScoped[LocalSDK]
}

func (p *prepareSDK) Action() *tasks.ActionEvent {
	return tasks.Action("go.sdk.prepare").Arg("version", p.version)
}
func (p *prepareSDK) Inputs() *compute.In {
	return compute.Inputs().Str("version", p.version).JSON("ref", p.ref)
}
func (p *prepareSDK) Output() compute.Output { return compute.Output{NotCacheable: true} }
func (p *prepareSDK) Compute(ctx context.Context, _ compute.Resolved) (LocalSDK, error) {
	// XXX security
	// We only checksum go/bin/go, it's a robustness/performance trade-off.
	fsys := unpack.Unpack("go-sdk", tarfs.TarGunzip(download.URL(p.ref)), unpack.WithChecksumPaths("go/bin/go"))

	// The contents of the sdk are unpacked here, rather than as an input to
	// this computable, as DoScoped Computables must have a deterministic set of
	// inputs; and the digest of a FS is only known after the FS is available.
	sdk, err := compute.GetValue(ctx, fsys)
	if err != nil {
		return LocalSDK{}, err
	}

	goBin := filepath.Join(sdk.Files, "go/bin/go")
	if !isNixOS() {
		return LocalSDK{
			Path:    sdk.Files,
			Version: p.version,
			goBin:   goBin,
		}, nil
	}

	patchedGoBin, err := ensureNixosPatched(ctx, goBin)
	if err != nil {
		return LocalSDK{}, err
	}

	return LocalSDK{
		Path:    sdk.Files,
		Version: p.version,
		goBin:   patchedGoBin,
	}, nil
}

func ensureNixosPatched(ctx context.Context, goBin string) (string, error) {
	// We keep a symlink to the interpreter go/bin/go was patched with. If the link
	// exists, then the interpreter exists, and no additional patching is required.
	dynlink := goBin + nixosDynsym
	patchedGoBin := goBin + ".patched"

	if _, err := os.Readlink(dynlink); err == nil {
		return patchedGoBin, nil
	}

	return tasks.Return(ctx, tasks.Action("nixos.patch-elf").Arg("path", goBin), func(ctx context.Context) (string, error) {
		// Assume all NixOS versions have systemd, which is true for all recent versions.
		systemCtl, err := output(ctx, "which", "systemctl")
		if err != nil {
			return "", err
		}

		existingInterpreter, err := output(ctx, "nix-shell", "-p", "patchelf", "--run", fmt.Sprintf("patchelf --print-interpreter %s", systemCtl))
		if err != nil {
			return "", err
		}

		cint := strings.TrimSpace(string(existingInterpreter))

		if _, err := output(ctx, "nix-shell", "-p", "patchelf", "--run", fmt.Sprintf("patchelf --set-interpreter %s --output %s %s", cint, patchedGoBin, goBin)); err != nil {
			return "", err
		}

		if err := os.Remove(dynlink); err != nil && os.IsNotExist(err) {
			fmt.Fprintf(console.Errors(ctx), "Failed to remove %s: %v", dynlink, err)
		}

		// Remember which interpreter we used, and if it doesn't exist, re-patch.
		if err := os.Symlink(cint, dynlink); err != nil {
			return "", err
		}

		return patchedGoBin, nil
	})
}

type LocalSDK struct {
	Path    string
	Version string

	goBin string
}

var _ compute.Digestible = LocalSDK{}

func (sdk LocalSDK) GoRoot() string { return filepath.Join(sdk.Path, "go") }
func (sdk LocalSDK) GoBin() string  { return sdk.goBin }

func (sdk LocalSDK) GoRootEnv() string {
	return fmt.Sprintf("GOROOT=%s", sdk.GoRoot())
}

func (sdk LocalSDK) ComputeDigest(context.Context) (schema.Digest, error) {
	return schema.DigestOf(sdk)
}

func isNixOS() bool {
	_, err := os.Stat("/etc/NIXOS")
	return err == nil
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
