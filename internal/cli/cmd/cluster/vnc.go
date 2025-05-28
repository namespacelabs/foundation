// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/framework/netcopy"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewVncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vnc [instance-id]",
		Short: "Start a VNC session.",
		Args:  cobra.ArbitraryArgs,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, err := selectClusterFriendly(ctx, args)
		if err != nil {
			return err
		}
		if cluster == nil {
			return nil
		}

		return connectToInstanceService(ctx, cluster, "vnc", func(addr string, creds *api.Cluster_ServiceState_Credentials) error {
			fmt.Fprint(console.Stdout(ctx), "Opening VNC client...\n")

			up := "admin:admin"
			if creds != nil {
				up = creds.Username + ":" + creds.Password
			}

			url := fmt.Sprintf("vnc://%s@%s", up, addr)
			return browser.OpenURL(url)
		})
	})

	return cmd
}

func connectToInstanceService(ctx context.Context, instance *api.KubernetesCluster, service string,
	use func(addr string, creds *api.Cluster_ServiceState_Credentials) error) error {
	svc := api.ClusterService(instance, service)
	if svc == nil || svc.Endpoint == "" {
		return fnerrors.Newf("instance does not have %s", service)
	}

	if svc.Status != "READY" {
		return fnerrors.Newf("expected vnc to be READY, saw %q", svc.Status)
	}

	peerConn, err := api.DialEndpoint(ctx, svc.Endpoint)
	if err != nil {
		return err
	}

	fmt.Fprint(console.Stdout(ctx), "Connected to instance.\n")

	defer peerConn.Close()

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	var d net.ListenConfig
	lst, err := d.Listen(ctxWithCancel, "tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	eg, _ := errgroup.WithContext(ctxWithCancel)
	eg.Go(func() error {
		conn, err := lst.Accept()
		if err != nil {
			return err
		}

		fmt.Fprint(console.Stdout(ctx), "Client connected.\n")
		_ = netcopy.CopyConns(nil, conn, peerConn)
		fmt.Fprint(console.Stdout(ctx), "Client disconnected, leaving.\n")
		return nil
	})

	if err := use(lst.Addr().String(), svc.Credentials); err != nil {
		return err
	}

	return eg.Wait()
}

func selectClusterFriendly(ctx context.Context, args []string) (*api.KubernetesCluster, error) {
	cluster, _, err := SelectRunningCluster(ctx, args)
	if errors.Is(err, ErrEmptyClusterList) {
		PrintCreateClusterMsg(ctx)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return cluster, err
}
