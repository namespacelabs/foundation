// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
)

func NewExecScoped() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec-scoped --service=docker|kubernetes <clusterid> <cmd...>",
		Short: "Runs the specified command (e.g. a script) with the corresponding environment variables set, based on the services selected (e.g. DOCKER_HOST, KUBECONFIG).",
		Args:  cobra.MinimumNArgs(1),
	}

	service := cmd.Flags().StringSlice("service", nil, "Which services to inject, any of: docker, kubernetes")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusterId := args[0]
		command := args[1:]

		if len(command) == 0 {
			return fnerrors.New("at least one command is required")
		}

		svcs := unique(*service)
		if len(svcs) == 0 {
			return fnerrors.New("at least one --service is required")
		}

		response, err := api.EnsureCluster(ctx, api.Methods, clusterId)
		if err != nil {
			return err
		}

		var injected []injected

		defer func() {
			for _, inj := range injected {
				inj.Cleanup()
			}
		}()

		for _, svc := range svcs {
			if constructor, ok := constructors[svc]; ok {
				inj, err := constructor(ctx, response.Cluster)
				if err != nil {
					return err
				}
				injected = append(injected, inj)
			} else {
				return fnerrors.New("no such service %q", svc)
			}
		}

		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Env = os.Environ()
		for _, inj := range injected {
			cmd.Env = append(cmd.Env, inj.Env...)
		}
		return localexec.RunInteractive(ctx, cmd)
	})

	return cmd
}

type injected struct {
	Env     []string
	Cleanup func()
}

var constructors = map[string]func(context.Context, *api.KubernetesCluster) (injected, error){
	"docker": func(ctx context.Context, cluster *api.KubernetesCluster) (injected, error) {
		p, err := runUnixSocketProxy(ctx, cluster.ClusterId, unixSockProxyOpts{
			Kind: "docker",
			Connect: func(ctx context.Context) (net.Conn, error) {
				token, err := fnapi.FetchToken(ctx)
				if err != nil {
					return nil, err
				}
				return connectToDocker(ctx, token, cluster)
			},
		})
		if err != nil {
			return injected{}, err
		}

		return injected{
			Env: []string{
				"DOCKER_HOST=unix://" + p.SocketAddr,
			},
			Cleanup: p.Cleanup,
		}, nil
	},

	"kubernetes": func(ctx context.Context, cluster *api.KubernetesCluster) (injected, error) {
		response, err := api.GetKubernetesConfig(ctx, api.Methods, cluster.ClusterId)
		if err != nil {
			return injected{}, err
		}

		cfg, err := kubectl.WriteKubeconfig(ctx, []byte(response.Kubeconfig), true)
		if err != nil {
			return injected{}, err
		}

		return injected{
			Env: []string{
				"KUBECONFIG=" + cfg.Kubeconfig,
			},
			Cleanup: func() {
				_ = os.Remove(cfg.Kubeconfig)
			},
		}, nil
	},
}

func unique(strs []string) []string {
	m := map[string]struct{}{}
	for _, str := range strs {
		m[strings.ToLower(str)] = struct{}{}
	}
	keys := maps.Keys(m)
	slices.Sort(keys)
	return keys
}
