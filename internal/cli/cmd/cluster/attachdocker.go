// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/context/store"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newDockerAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach-context",
		Short: "Configures a Docker context that uses an ephemeral environment.",
		Args:  cobra.NoArgs,
	}

	name := cmd.Flags().String("name", "", "The name of the context that is created; by default it will be nsc-{id}.")
	use := cmd.Flags().Bool("use", false, "If true, set the new context as the default.")
	stateDir := cmd.Flags().String("state", "", "If set, stores the proxy socket in this directory.")
	toCluster := cmd.Flags().String("to", "", "Attaches a context to the specified instance.")
	new := cmd.Flags().Bool("new", false, "If set, creates a new instance.")
	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type.")
	background := cmd.Flags().Bool("background", false, "If set, attach in the background.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if !*new && *toCluster == "" {
			return fnerrors.New("one of --new or --to is required")
		} else if *new && *toCluster != "" {
			return fnerrors.New("only one of --new or --to may be specified")
		}

		dockerCli, err := command.NewDockerCli()
		if err != nil {
			return err
		}

		if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
			return err
		}

		if *name != "" {
			if err := store.ValidateContextName(*name); err != nil {
				return err
			}
		}

		cluster, err := ensureDockerCluster(ctx, *toCluster, *machineType, *background)
		if err != nil {
			return err
		}

		state, err := ensureStateDir(*stateDir, "docker/"+cluster.ClusterId)
		if err != nil {
			return err
		}

		eg := executor.New(ctx, "docker")

		sockPath := filepath.Join(state, "docker.sock")

		if *background {
			if err := setupBackgroundProxy(ctx, cluster.ClusterId, "docker", sockPath, ""); err != nil {
				return err
			}
		} else {
			eg.Go(func(ctx context.Context) error {
				// if *new {
				// 	defer func() {
				// 		_ = api.Endpoint.ReleaseKubernetesCluster.Do(ctx, api.ReleaseKubernetesClusterRequest{
				// 			ClusterId: cluster.ClusterId,
				// 		}, nil)
				// 	}()
				// }

				_, err := runUnixSocketProxy(ctx, cluster.ClusterId, unixSockProxyOpts{
					SocketPath: sockPath,
					Kind:       "docker",
					Blocking:   true,
					Connect: func(ctx context.Context) (net.Conn, error) {
						token, err := fnapi.FetchToken(ctx)
						if err != nil {
							return nil, err
						}

						return connectToDocker(ctx, token, cluster)
					},
				})
				return err
			})
		}

		ctxName := *name
		if ctxName == "" {
			ctxName = "nsc-" + cluster.ClusterId
		}

		eg.Go(func(ctx context.Context) error {
			s := dockerCli.ContextStore()

			md := store.Metadata{
				Endpoints: map[string]interface{}{
					docker.DockerEndpoint: docker.EndpointMeta{
						Host: "unix://" + sockPath,
					},
				},
				Metadata: command.DockerContext{
					Description: fmt.Sprintf("Namespace-managed Docker environment (hosted on %s)", cluster.ClusterId),
				},
				Name: ctxName,
			}

			if err := s.CreateOrUpdate(md); err != nil {
				return err
			}

			console.SetStickyContent(ctx, "docker", dockerBanner(ctx, ctxName, *use, *background))

			was := dockerCli.CurrentContext()

			if !*background {
				eg.Go(func(ctx context.Context) error {
					<-ctx.Done() // Wait for cancelation.

					removeErr := s.Remove(ctxName)
					if *use {
						if err := updateContext(dockerCli, was, func(current string) bool {
							return current == ctxName
						}); err != nil {
							return multierr.New(removeErr, err)
						}
					}

					return removeErr
				})
			}

			if *use {
				if err := updateContext(dockerCli, ctxName, nil); err != nil {
					return err
				}
			}

			return nil
		})

		return eg.Wait()
	})

	return cmd
}

func dockerBanner(ctx context.Context, ctxName string, use, background bool) string {
	w := wordwrap.NewWriter(80)
	style := colors.Ctx(ctx)

	fmt.Fprintf(w, "Attached Docker Context: %s\n\n", ctxName)

	if use {
		fmt.Fprintf(w, "You can use the context directly:\n\n")
		fmt.Fprintf(w, "  $ docker run --rm -it ubuntu\n")
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "You can use the context by passing `--context %s` to `docker`:\n\n", ctxName)
		fmt.Fprintf(w, "  $ docker --context %s run --rm -it ubuntu\n", ctxName)
		fmt.Fprintln(w)

	}

	fmt.Fprintf(w, "Or by setting the environment variable DOCKER_CONTEXT:\n\n")
	fmt.Fprintf(w, "  $ export DOCKER_CONTEXT=%q\n", ctxName)
	fmt.Fprintf(w, "  $ docker run --rm -it ubuntu\n")

	if !background {
		fmt.Fprintln(w)
		fmt.Fprintln(w, style.Comment.Apply("Exiting will remove the context configuration."))
	}

	_ = w.Close()
	return strings.TrimSpace(w.String())
}

func updateContext(dockerCli *command.DockerCli, ctxName string, shouldUpdate func(string) bool) error {
	dockerConfig := dockerCli.ConfigFile()

	if shouldUpdate == nil {
		shouldUpdate = func(current string) bool {
			return current != ctxName
		}
	}

	// Avoid updating the config-file if nothing changed. This also prevents
	// creating the file and config-directory if the default is used and
	// no config-file existed yet.
	if shouldUpdate(dockerConfig.CurrentContext) {
		dockerConfig.CurrentContext = ctxName
		if err := dockerConfig.Save(); err != nil {
			return err
		}
	}

	return nil
}

func ensureDockerCluster(ctx context.Context, instanceId, machineType string, background bool) (*api.KubernetesCluster, error) {
	if instanceId != "" {
		resp, err := api.EnsureCluster(ctx, api.Methods, nil, instanceId)
		if err != nil {
			return nil, err
		}

		return resp.Cluster, nil
	}

	featuresList := []string{"EXP_DISABLE_KUBERNETES"}
	resp, err := api.CreateAndWaitCluster(ctx, api.Methods, api.CreateClusterOpts{
		Purpose:     "Docker environment",
		Features:    featuresList,
		KeepAtExit:  background,
		MachineType: machineType,
		WaitClusterOpts: api.WaitClusterOpts{
			CreateLabel:    "Creating Docker environment",
			WaitForService: "buildkit",
			WaitKind:       "buildcluster",
		},
	})
	if err != nil {
		return nil, err
	}

	return resp.Cluster, nil
}
