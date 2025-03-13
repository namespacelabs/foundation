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
		Use:    "vnc [instance-id]",
		Short:  "Start a VNC session.",
		Args:   cobra.ArbitraryArgs,
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := SelectRunningCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusterList) {
				PrintCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		return runVnc(ctx, cluster)
	})

	return cmd
}

func runVnc(ctx context.Context, instance *api.KubernetesCluster) error {
	vncSvc := api.ClusterService(instance, "vnc")
	if vncSvc == nil || vncSvc.Endpoint == "" {
		return fnerrors.Newf("instance does not have vnc")
	}

	if vncSvc.Status != "READY" {
		return fnerrors.Newf("expected vnc to be READY, saw %q", vncSvc.Status)
	}

	peerConn, err := api.DialEndpoint(ctx, vncSvc.Endpoint)
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

		fmt.Fprint(console.Stdout(ctx), "VNC client connected.\n")
		_ = netcopy.CopyConns(nil, conn, peerConn)
		fmt.Fprint(console.Stdout(ctx), "VNC client disconnected, leaving.\n")
		return nil
	})

	fmt.Fprint(console.Stdout(ctx), "Opening VNC client...\n")
	// XXX receive credentials.
	if err := browser.OpenURL(fmt.Sprintf("vnc://admin:admin@%s", lst.Addr())); err != nil {
		return err
	}

	return eg.Wait()
}
