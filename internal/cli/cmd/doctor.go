// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/kr/text"
	"github.com/moby/buildkit/client/llb"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/compute/cache"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

const (
	checkTimeLimit = 30 * time.Second
)

type DoctorResults struct {
	NSVersion     *VersionInfo                    `json:"ns"`
	NSBootVersion *storage.NamespaceBinaryVersion `json:"nsboot"`
	DockerInfo    *DoctorResults_DockerInfo       `json:"docker_info"`
	DockerRun     *DoctorResults_DockerRun        `json:"docker_run"`
	Buildkit      *DoctorResults_BuildkitResults  `json:"buildkit"`
	KubeResults   *DoctorResults_KubeResults      `json:"kubernetes"`
}

type DoctorResults_DockerInfo struct {
	dockertypes.Version
	dockertypes.Info
}

type DoctorResults_DockerRun struct {
	ImageLatency time.Duration `json:"image_latency"`
	RunLatency   time.Duration `json:"run_latency"`
}

type DoctorResults_BuildkitResults struct {
	BuildLatency time.Duration `json:"build_latency"`
	ImageDigest  string        `json:"image_digest"`
}

type DoctorResults_KubeResults struct {
	ConnectLatency time.Duration `json:"connect_latency"`
	RunLatency     time.Duration `json:"run_latency"`
}

func NewDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Collect diagnostic information about the system.",
		Args:  cobra.NoArgs,
	}

	var results DoctorResults

	testFilter := cmd.Flags().StringSlice("tests", nil, "If set, filters which tests are run.")
	uploadResults := cmd.Flags().Bool("upload_results", !environment.IsRunningInCI(), "If set, anonymized results are pushed to the Namespace team.")
	envRef := cmd.Flags().String("env", "dev", "The environment to test.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		var errCount int

		if filterIncludes(testFilter, "version") {
			versionI := runDiagnostic(ctx, "doctor.version-info", func(ctx context.Context) (*VersionInfo, error) {
				return CollectVersionInfo()
			})
			printDiagnostic(ctx, "Namespace version", versionI, &errCount, func(w io.Writer, vi *VersionInfo) {
				results.NSVersion = vi
				FormatVersionInfo(w, vi)
			})
		}

		if filterIncludes(testFilter, "nsboot-version") {
			nsbootI := runDiagnostic(ctx, "doctor.nsboot-version", func(ctx context.Context) (*storage.NamespaceBinaryVersion, error) {
				v, err := nsboot.GetBootVersion()
				if err != nil {
					return nil, err
				}
				if v == nil {
					return nil, fnerrors.New("not running nsboot")
				}
				return v, nil
			})
			printDiagnostic(ctx, "NSBoot version", nsbootI, &errCount, func(w io.Writer, vi *storage.NamespaceBinaryVersion) {
				results.NSBootVersion = vi
				FormatBinaryVersion(w, vi)
			})
		}

		if filterIncludes(testFilter, "docker-info") {
			dockerI := runDiagnostic(ctx, "doctor.docker-info", func(ctx context.Context) (DoctorResults_DockerInfo, error) {
				client, err := docker.NewClient()
				if err != nil {
					return DoctorResults_DockerInfo{}, err
				}
				version, err := client.ServerVersion(ctx)
				if err != nil {
					return DoctorResults_DockerInfo{}, err
				}
				info, err := client.Info(ctx)
				if err != nil {
					return DoctorResults_DockerInfo{}, err
				}
				return DoctorResults_DockerInfo{version, info}, nil
			})
			printDiagnostic(ctx, "Docker", dockerI, &errCount, func(w io.Writer, info DoctorResults_DockerInfo) {
				info.ID = "" // Clear IDs.
				results.DockerInfo = &info
				fmt.Fprintf(w, "version=%s (commit=%s) for %s-%s\n", info.Version.Version, info.Version.GitCommit, info.Version.Os, info.Version.Arch)
				fmt.Fprintf(w, "api version=%s (min=%s)\n", info.Version.APIVersion, info.Version.MinAPIVersion)
				fmt.Fprintf(w, "kernel version=%s\n", info.Version.KernelVersion)
				fmt.Fprintf(w, "ncpu=%d mem_total=%d\n", info.Info.NCPU, info.Info.MemTotal)
				fmt.Fprintf(w, "containers total=%d running=%d paused=%d stopped=%d images=%d\n", info.Info.Containers, info.Info.ContainersRunning,
					info.Info.ContainersPaused, info.Info.ContainersStopped, info.Info.Images)
				fmt.Fprintf(w, "driver=%s logging_driver=%s cgroup_driver=%s cgroup_version=%s\n", info.Info.Driver, info.Info.LoggingDriver,
					info.Info.CgroupDriver, info.Info.CgroupVersion)
				fmt.Fprintf(w, "containerd expected=%s present=%s\n", info.Info.ContainerdCommit.Expected, info.Info.ContainerdCommit.ID)
				fmt.Fprintf(w, "runc expected=%s present=%s\n", info.Info.RuncCommit.Expected, info.Info.RuncCommit.ID)
				fmt.Fprintf(w, "init expected=%s present=%s\n", info.Info.InitCommit.Expected, info.Info.InitCommit.ID)
				if len(info.Info.Warnings) > 0 {
					fmt.Fprintln(w, "Warnings:")
					for _, wn := range info.Info.Warnings {
						fmt.Fprintf(w, "  %s", wn)
					}
				}
			})
		}

		if filterIncludes(testFilter, "userauth") {
			loginI := runDiagnostic(ctx, "doctor.userauth", func(ctx context.Context) (*fnapi.UserAuth, error) {
				return fnapi.LoadUser()
			})
			printDiagnostic(ctx, "Authenticated User", loginI, &errCount, func(w io.Writer, info *fnapi.UserAuth) {
				fmt.Fprintf(w, "user=%s\n", info.Username)
			})
		}

		if filterIncludes(testFilter, "docker-run") {
			dockerRunI := runDiagnostic(ctx, "doctor.docker-run", func(ctx context.Context) (DoctorResults_DockerRun, error) {
				t := time.Now()
				image, err := compute.GetValue(ctx, oci.ResolveImage(pins.Image("hello-world"), docker.HostPlatform()).Image())
				if err != nil {
					return DoctorResults_DockerRun{}, err
				}
				var r DoctorResults_DockerRun
				r.ImageLatency = time.Since(t)
				t = time.Now()
				err = docker.Runtime().Run(ctx, rtypes.RunToolOpts{
					RunBinaryOpts: rtypes.RunBinaryOpts{
						Image: image,
					},
				})
				r.RunLatency = time.Since(t)
				return r, err
			})

			printDiagnostic(ctx, "Docker Run Check", dockerRunI, &errCount, func(w io.Writer, info DoctorResults_DockerRun) {
				results.DockerRun = &info
				fmt.Fprintf(w, "image_pull_latency=%v docker_run_latency=%v\n", info.ImageLatency, info.RunLatency)
			})
		}

		if filterIncludes(testFilter, "workspace") {
			workspaceI := runDiagnostic(ctx, "doctor.workspace", func(ctx context.Context) (*parsing.Root, error) {
				return module.FindRoot(ctx, ".")
			})
			printDiagnostic(ctx, "Workspace", workspaceI, &errCount, func(w io.Writer, ws *parsing.Root) {
				fmt.Fprintf(w, "module=%s\n", ws.ModuleName())
			})

			if filterIncludes(testFilter, "buildkit-build") {
				buildkitI := errorOr[DoctorResults_BuildkitResults]{err: fnerrors.New("no workspace")}
				if workspaceI.err == nil {
					buildkitI = runDiagnostic(ctx, "doctor.build", func(ctx context.Context) (DoctorResults_BuildkitResults, error) {
						env, err := cfg.LoadContext(workspaceI.v, *envRef)
						if err != nil {
							return DoctorResults_BuildkitResults{}, err
						}
						hostPlatform := docker.HostPlatform()
						state := llb.Scratch().File(llb.Mkfile("/hello", 0644, []byte("cachehit")))
						conf := build.NewBuildTarget(&hostPlatform).WithSourceLabel("doctor.hello-world")
						var r DoctorResults_BuildkitResults
						t := time.Now()
						imageC, err := buildkit.BuildImage(ctx, buildkit.DeferClient(env.Configuration(), &hostPlatform), conf, state)
						if err != nil {
							return DoctorResults_BuildkitResults{}, err
						}
						image, err := compute.Get(ctx, imageC)
						if err != nil {
							return DoctorResults_BuildkitResults{}, err
						}
						r.BuildLatency = time.Since(t)
						if digest, err := image.Value.Digest(); err == nil {
							r.ImageDigest = digest.String()
						}
						return r, nil
					})
				}
				printDiagnostic(ctx, "Buildkit", buildkitI, &errCount, func(w io.Writer, r DoctorResults_BuildkitResults) {
					results.Buildkit = &r
					fmt.Fprintf(w, "success build_latency=%v image_digest=%v\n", r.BuildLatency, r.ImageDigest)
				})
			}

			if filterIncludes(testFilter, "kubernetes-run") {
				kubernetesI := errorOr[DoctorResults_KubeResults]{err: fnerrors.New("no workspace")}
				if workspaceI.err == nil {
					kubernetesI = runDiagnostic(ctx, "doctor.kube", func(ctx context.Context) (DoctorResults_KubeResults, error) {
						var r DoctorResults_KubeResults
						env, err := cfg.LoadContext(workspaceI.v, *envRef)
						if err != nil {
							return r, err
						}
						t := time.Now()
						k, err := kubernetes.ConnectToCluster(ctx, env.Configuration())
						if err != nil {
							return r, err
						}
						r.ConnectLatency = time.Since(t)
						helloID, err := oci.ParseImageID(pins.Image("hello-world"))
						if err != nil {
							return r, err
						}
						t = time.Now()
						err = k.RunAttachedOpts(ctx, nil, "default", "doctor-"+ids.NewRandomBase32ID(8),
							runtime.ContainerRunOpts{Image: helloID}, runtime.TerminalIO{}, func() {})
						r.RunLatency = time.Since(t)

						return r, err
					})
				}

				printDiagnostic(ctx, "Kubernetes Run Check", kubernetesI, &errCount, func(w io.Writer, info DoctorResults_KubeResults) {
					results.KubeResults = &info
					fmt.Fprintf(w, "connect_latency=%v run_latency=%v\n", info.ConnectLatency, info.RunLatency)
				})
			}
		}

		out := console.TypedOutput(ctx, "Support", common.CatOutputUs)
		fmt.Fprintf(out, "\nHaving trouble? Chat with us at https://community.namespace.so/discord\n")

		if *uploadResults {
			serialized, err := json.Marshal(results)
			if err == nil {
				var response recordDoctorResponse
				if err := (fnapi.Call[recordDoctorRequest]{
					Endpoint:  fnapi.EndpointAddress,
					Method:    "nsl.support.SupportService/RecordDoctor",
					Anonymous: true,
				}).Do(ctx, recordDoctorRequest{ResultsBlob: serialized}, func(r io.Reader) error {
					return json.NewDecoder(r).Decode(&response)
				}); err != nil {
					fmt.Fprintf(console.Warnings(ctx), "Failed to push results: %v\n", err)
				} else if response.InvocationId != "" {
					fmt.Fprintf(out, "\nAnd please refer to %q (your anonymized results) so we can more quickly help you.\n", response.InvocationId)
				}
			} else {
				fmt.Fprintf(console.Warnings(ctx), "Failed to serialized results: %v\n", err)
			}
		}

		if errCount > 0 {
			return fnerrors.ExitWithCode(fmt.Errorf("%d tests failed", errCount), 1)
		}

		return nil
	})
}

func filterIncludes(filter *[]string, key string) bool {
	return len(*filter) == 0 || slices.Contains(*filter, strings.ToLower(key))
}

func runDiagnostic[V any](ctx context.Context, title string, f func(ctx context.Context) (V, error)) errorOr[V] {
	res := errorOr[V]{}
	// We have to run in a separate orchestrator so that failures in one diagnostic
	// do not prevent other diagnostics from proceeding.
	res.err = compute.DoWithCache(ctx, cache.NoCache, func(ctx context.Context) error {
		v, err := tasks.Return(ctx, tasks.Action(title), func(ctx context.Context) (V, error) {
			timedCtx, cancel := context.WithTimeout(ctx, checkTimeLimit)
			defer cancel()
			return f(timedCtx)
		})
		res.v = v
		return err
	})
	return res
}

func printDiagnostic[V any](ctx context.Context, title string, res errorOr[V], errCount *int, print func(io.Writer, V)) {
	style := colors.Ctx(ctx)

	w := console.TypedOutput(ctx, title, common.CatOutputUs)

	fmt.Fprintln(w, style.Header.Apply(fmt.Sprintf("* %s", title)))
	x := text.NewIndentWriter(w, []byte("  "))
	if res.err != nil {
		// Not using format.Format since it's too verbose.
		fmt.Fprintln(x, style.ErrorHeader.Apply("Failed:"), res.err)
		*errCount++
	} else {
		print(x, res.v)
	}
}

type errorOr[V any] struct {
	v   V
	err error
}

type recordDoctorRequest struct {
	ResultsBlob []byte `protobuf:"bytes,1,opt,name=results_blob,json=resultsBlob,proto3" json:"results_blob,omitempty"`
}

type recordDoctorResponse struct {
	InvocationId string `protobuf:"bytes,1,opt,name=invocation_id,json=invocationId,proto3" json:"invocation_id,omitempty"`
}
