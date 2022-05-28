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
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/disk"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const version = "5.4.1"

var IgnoreZfsCheck = false

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/rancher/k3d/releases/download/v%s/k3d-linux-amd64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "50f64747989dc1fcde5db5cb82f8ac132a174b607ca7dfdb13da2f0e509fda11",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://github.com/rancher/k3d/releases/download/v%s/k3d-linux-arm64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "712ffb338ec1fed6f7c1c8691b79bc80967cc17fef256cd620190d0d7e502052",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://github.com/rancher/k3d/releases/download/v%s/k3d-darwin-arm64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "ddb71b5963ee2d34aa4aa78a49e99a0d4bacd5db61f16e071b99d3a846afe2dc",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/rancher/k3d/releases/download/v%s/k3d-darwin-amd64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "1e008f1661a5c939cb9a6991b612ee51dd7080e6e2b1443065f3d522378e50a4",
		},
	},
}

type State struct {
	Running bool   `json:"running,omitempty"`
	Status  string `json:"status,omitempty"`
}

type Node struct {
	Name  string `json:"name,omitempty"`
	Role  string `json:"role,omitempty"`
	State State  `json:"state,omitempty"`
}

type Cluster struct {
	Name  string `json:"name,omitempty"`
	Nodes []Node `json:"nodes,omitempty"`
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

	w := unpack.Unpack("k3d", unpack.MakeFilesystem("k3d", 0755, ref))

	return compute.Map(
		tasks.Action("k3d.ensure").Arg("version", version).HumanReadablef("Ensuring k3d %s is installed", version),
		compute.Inputs().Computable("k3d", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (K3D, error) {
			return K3D(filepath.Join(compute.MustGetDepValue(r, w, "k3d").Files, "k3d")), nil
		}), nil
}

func AllDownloads() []compute.Computable[bytestream.ByteStream] {
	var downloads []compute.Computable[bytestream.ByteStream]
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
		return fnerrors.InvocationError("failed to obtain docker version: %w", err)
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

// If port is 0, an open port is allocated dynamically.
func (k3d K3D) CreateRegistry(ctx context.Context, name string, port int) error {
	if !strings.HasPrefix(name, "k3d-") {
		return fnerrors.UserError(nil, "a k3d- prefix is required in registry names")
	}

	return tasks.Action("k3d.create-image-registry").Run(ctx, func(ctx context.Context) error {
		args := []string{"registry", "create", strings.TrimPrefix(name, "k3d-")}
		if port != 0 {
			args = append(args, "-p", fmt.Sprintf("%d", port))
		}

		return k3d.do(ctx, args...)
	})
}

func (k3d K3D) CreateCluster(ctx context.Context, name, registry, image string, updateDefault bool) error {
	fmt.Fprintf(console.Stdout(ctx), "Creating a Kubernetes cluster, this may take up to a minute (image=%s).\n", image)

	return tasks.Action("k3d.create-cluster").Arg("image", image).Run(ctx, func(ctx context.Context) error {
		return k3d.do(ctx, "cluster", "create", "--registry-use", registry, "--image", image, fmt.Sprintf("--kubeconfig-update-default=%v", updateDefault), "--k3s-arg", "--disable=traefik@server:0", "--wait", name)
	})
}

func (k3d K3D) MergeConfiguration(ctx context.Context, name string) error {
	return tasks.Action("k3d.merge-configuration").Arg("name", name).Run(ctx, func(ctx context.Context) error {
		return k3d.do(ctx, "kubeconfig", "merge", name, "-d", "--kubeconfig-switch-context=false")
	})
}

func (k3d K3D) StartNode(ctx context.Context, nodeName string) error {
	return tasks.Action("k3d.start-node").Arg("name", nodeName).Run(ctx, func(ctx context.Context) error {
		return k3d.do(ctx, "node", "start", nodeName)
	})
}

func (k3d K3D) StopNode(ctx context.Context, nodeName string) error {
	return tasks.Action("k3d.stop-node").Arg("name", nodeName).Run(ctx, func(ctx context.Context) error {
		return k3d.do(ctx, "node", "stop", nodeName)
	})
}

func (k3d K3D) do(ctx context.Context, args ...string) error {

	cmd := exec.CommandContext(ctx, string(k3d), args...)
	cmd.Stdout = console.Output(ctx, "k3d")
	cmd.Stderr = console.Output(ctx, "k3d")
	return localexec.RunAndPropagateCancelation(ctx, "k3d", cmd)
}
