// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package k3d

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/disk"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const version = "5.2.2"

var IgnoreZfsCheck = false

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/rancher/k3d/releases/download/v%s/k3d-linux-amd64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "7ddb900e6e50120b65d61568f6af007a82331bf83918608a6a7be8910792faef",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://github.com/rancher/k3d/releases/download/v%s/k3d-linux-arm64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "ccf1dafc1eddfef083375377a52ef0ca269a41c5bc4f0f4d7e11a7c56da08833",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://github.com/rancher/k3d/releases/download/v%s/k3d-darwin-arm64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "d0149ecb9b3fb831d617a0a880d8235722a70b9131f45f1389235e586050f8f9",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/rancher/k3d/releases/download/v%s/k3d-darwin-amd64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "40ac312bc762611de80daff24cb66d79aaaf17bf90e5e8d61caf90e63b57542d",
		},
	},
}

type Cluster struct {
	Name string `json:"name"`
}

type K3D string

func EnsureSDK(ctx context.Context) (K3D, error) {
	sdk, err := SDK(ctx)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context) (compute.Computable[K3D], error) {
	platform := devhost.RuntimePlatform()
	key := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.UserError(nil, "platform not supported: %s", key)
	}

	if !IgnoreZfsCheck {
		if fstype, err := disk.FSType("/"); err != nil {
			fmt.Fprintf(console.Warnings(ctx), "failed to retrieve filesystem type, can't check for ZFS: %v\n", err)
		} else if fstype == "zfs" {
			return nil, fnerrors.InternalError("currently a base system of ZFS is not supported, as it is not compatible with k3d (see https://github.com/namespacelabs/foundation/issues/121). You can ignore this check by retrying with --ignore_zfs_check")
		}
	}

	cacheDir, err := dirs.SDKCache("k3d")
	if err != nil {
		return nil, err
	}

	k3dPath := filepath.Join(cacheDir, "k3d")
	written := unpack.WriteLocal(k3dPath, 0755, ref)

	return compute.Map(
		tasks.Action("k3d.ensure").Arg("version", version).HumanReadablef("Ensuring k3d %s is installed", version),
		compute.Inputs().Computable("k3d", written),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (K3D, error) {
			return K3D(compute.GetDepValue(r, written, "k3d")), nil
		}), nil
}

func AllDownloads() []compute.Computable[compute.ByteStream] {
	var downloads []compute.Computable[compute.ByteStream]
	for _, v := range Pins {
		downloads = append(downloads, download.URL(v))
	}
	return downloads
}

// https://github.com/rancher/k3d/issues/807
const minimumDockerVer = "20.10.5"
const minimumRuncVer = "1.0.0-rc93"

func ValidateDocker(ctx context.Context, cli docker.Client) error {
	ver, err := cli.ServerVersion(ctx)
	if err != nil {
		return fnerrors.RemoteError("failed to obtain docker version: %w", err)
	}

	dockerOK, runcOK, runcVersion := validateVersions(ver)
	if !dockerOK || !runcOK {
		stderr := console.Stderr(ctx)
		fmt.Fprintln(stderr, "Docker does not meet our minimum requirements:")
		fmt.Fprintf(stderr, "  Docker meets: %v, minimum: %s, running: %s\n", dockerOK, minimumDockerVer, ver.Version)
		fmt.Fprintf(stderr, "  Runc meets: %v, minimum: %s, running: %s\n", runcOK, minimumRuncVer, runcVersion)
		return fnerrors.UserError(nil, "docker does not meet requirements")
	}

	return nil
}

func validateVersions(ver types.Version) (bool, bool, string) {
	dockerOK := semver.Compare("v"+ver.Version, "v"+minimumDockerVer) >= 0
	runcOK := false

	runcVersion := "<not present>"
	for _, comp := range ver.Components {
		if comp.Name == "runc" {
			runcVersion = comp.Version

			// Debian uses a different format for versions, using ~ instead of -
			// to mark rc builds. Ideally we'd have a more robust version parser
			// here, but for now, just convert all ~ back to - so Go's semver
			// parsing is happy.
			modifiedVersion := strings.ReplaceAll(runcVersion, "~", "-")

			runcOK = semver.Compare("v"+modifiedVersion, "v"+minimumRuncVer) >= 0
		}
	}
	return dockerOK, runcOK, runcVersion
}

func (k3d K3D) ListClusters(ctx context.Context) ([]Cluster, error) {
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, string(k3d), "cluster", "list", "-o", "json")
	cmd.Stdout = &output
	cmd.Stderr = console.Output(ctx, "k3d")
	if err := localexec.RunAndPropagateCancelation(ctx, "k3d", cmd); err != nil {
		return nil, err
	}

	var clusters []Cluster
	if err := json.Unmarshal(output.Bytes(), &clusters); err != nil {
		return nil, err
	}

	return clusters, nil
}

func (k3d K3D) CreateRegistry(ctx context.Context, name string, port int) error {
	if !strings.HasPrefix(name, "k3d-") {
		return fnerrors.UserError(nil, "a k3d- prefix is required in registry names")
	}

	return tasks.Action("k3d.create-image-registry").Run(ctx, func(ctx context.Context) error {
		return k3d.do(ctx, "registry", "create", strings.TrimPrefix(name, "k3d-"), "-p", fmt.Sprintf("%d", port))
	})
}

func (k3d K3D) CreateCluster(ctx context.Context, name, registry, image string, updateDefault bool) error {
	fmt.Fprintf(console.Stdout(ctx), "Creating a Kubernetes cluster, this may take up to a minute (image=%s).\n", image)

	return tasks.Action("k3d.create-cluster").Arg("image", image).Run(ctx, func(ctx context.Context) error {
		return k3d.do(ctx, "cluster", "create", "--registry-use", registry, "--image", image, fmt.Sprintf("--kubeconfig-update-default=%v", updateDefault), "--k3s-arg", "--disable=traefik@server:0", "--wait", name)
	})
}

func (k3d K3D) MergeConfiguration(ctx context.Context, name string) error {
	return k3d.do(ctx, "kubeconfig", "merge", name, "-d", "--kubeconfig-switch-context=false")
}

func (k3d K3D) do(ctx context.Context, args ...string) error {

	cmd := exec.CommandContext(ctx, string(k3d), args...)
	cmd.Stdout = console.Output(ctx, "k3d")
	cmd.Stderr = console.Output(ctx, "k3d")
	return localexec.RunAndPropagateCancelation(ctx, "k3d", cmd)
}
