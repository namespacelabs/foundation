// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/framework/netcopy"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

type proxyServiceCredentials struct {
	Credentials *proxyCredentials `json:"credentials,omitempty"`
}

type proxyCredentials struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type proxyOutput struct {
	Endpoint string
	Services map[string]*proxyServiceCredentials
}

func (p proxyOutput) MarshalJSON() ([]byte, error) {
	m := map[string]any{"endpoint": p.Endpoint}
	for k, v := range p.Services {
		m[k] = v
	}

	return json.Marshal(m)
}

func NewProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy [instance-id]",
		Short: "Provides proxy support for instance services.",
		Args:  cobra.ArbitraryArgs,
	}

	service := cmd.Flags().StringP("service", "s", "", "The service to proxy (e.g. vnc, rdp).")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	outputTo := cmd.Flags().String("output_to", "", "If specified, write the JSON configuration to this path.")
	once := cmd.Flags().Bool("once", false, "If set, stop the proxy after a single connection.")
	websocketProxy := cmd.Flags().Bool("websocket", false, "If set, outputs information to connect to the service directly using websockets.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *service == "" {
			return fnerrors.Newf("--service is required")
		}

		cluster, err := selectClusterFriendly(ctx, args)
		if err != nil {
			return err
		}
		if cluster == nil {
			return nil
		}

		svc := api.ClusterService(cluster, *service)
		if svc == nil || svc.Endpoint == "" {
			return fnerrors.Newf("instance does not have service %q", *service)
		}

		if svc.Status != "READY" {
			return fnerrors.Newf("expected %s to be READY, saw %q", *service, svc.Status)
		}

		if *websocketProxy {
			token, err := fnapi.FetchToken(ctx)
			if err != nil {
				return err
			}

			bt, err := token.IssueToken(ctx, 4*time.Hour, false)
			if err != nil {
				return err
			}

			targetURL, err := url.Parse(svc.Endpoint)
			if err != nil {
				return fnerrors.InternalError("failed to parse endpoint: %w", err)
			}

			q := targetURL.Query()
			q.Set("bearer_token", bt)
			targetURL.RawQuery = q.Encode()

			out := map[string]any{"url": targetURL.String()}
			if svc.Credentials != nil {
				out[*service] = &proxyServiceCredentials{
					Credentials: &proxyCredentials{
						Username: svc.Credentials.Username,
						Password: svc.Credentials.Password,
					},
				}
			}

			if *outputTo != "" {
				if err := files.WriteJson(*outputTo, out, 0600); err != nil {
					return fnerrors.InternalError("failed to write output to %q: %w", *outputTo, err)
				}
				fmt.Fprintf(console.Stdout(ctx), "Wrote %q\n", *outputTo)
			}

			switch *output {
			case "json":
				if err := json.NewEncoder(console.Stdout(ctx)).Encode(out); err != nil {
					return fnerrors.InternalError("failed to encode output as JSON: %w", err)
				}
			default:
				if *output != "" && *output != "plain" {
					fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", *output)
				}
				stdout := console.Stdout(ctx)
				fmt.Fprintf(stdout, "%s:\n", strings.ToUpper(*service))
				fmt.Fprintf(stdout, "  URL: %s\n", targetURL.String())
				if svc.Credentials != nil {
					fmt.Fprintf(stdout, "  Username: %s\n", svc.Credentials.Username)
					fmt.Fprintf(stdout, "  Password: %s\n", svc.Credentials.Password)
				}
			}

			return nil
		}

		var d net.ListenConfig
		lst, err := d.Listen(ctx, "tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		defer lst.Close()

		addr := lst.Addr().String()

		out := proxyOutput{
			Endpoint: addr,
		}
		if svc.Credentials != nil {
			out.Services = map[string]*proxyServiceCredentials{
				*service: {
					Credentials: &proxyCredentials{
						Username: svc.Credentials.Username,
						Password: svc.Credentials.Password,
					},
				},
			}
		}

		if *outputTo != "" {
			if err := files.WriteJson(*outputTo, out, 0600); err != nil {
				return fnerrors.InternalError("failed to write output to %q: %w", *outputTo, err)
			}

			fmt.Fprintf(console.Stdout(ctx), "Wrote %q\n", *outputTo)
		}

		switch *output {
		case "json":
			if err := json.NewEncoder(console.Stdout(ctx)).Encode(out); err != nil {
				return fnerrors.InternalError("failed to encode output as JSON: %w", err)
			}

		default:
			if *output != "" && *output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", *output)
			}

			stdout := console.Stdout(ctx)
			fmt.Fprintf(stdout, "%s:\n", strings.ToUpper(*service))
			fmt.Fprintf(stdout, "  Endpoint: %s\n", addr)
			if svc.Credentials != nil {
				fmt.Fprintf(stdout, "  Username: %s\n", svc.Credentials.Username)
				fmt.Fprintf(stdout, "  Password: %s\n", svc.Credentials.Password)
			}
		}

		eg, egCtx := errgroup.WithContext(ctx)

		handleConn := func(conn net.Conn) error {
			defer conn.Close()

			peerConn, err := api.DialEndpoint(egCtx, svc.Endpoint)
			if err != nil {
				if *output == "plain" {
					fmt.Fprintf(console.Warnings(ctx), "Failed to connect to service: %v\n", err)
				}

				return nil
			}

			defer peerConn.Close()

			if *output == "plain" {
				fmt.Fprintf(console.Stdout(ctx), "Client %s connected.\n", conn.RemoteAddr())
			}

			_ = netcopy.CopyConns(nil, conn, peerConn)

			if *output == "plain" {
				fmt.Fprintf(console.Stdout(ctx), "Client %s disconnected.\n", conn.RemoteAddr())
			}

			return nil
		}

		if *once {
			conn, err := lst.Accept()
			if err != nil {
				return err
			}

			return handleConn(conn)
		}

		eg.Go(func() error {
			for {
				conn, err := lst.Accept()
				if err != nil {
					if egCtx.Err() != nil {
						return nil
					}
					return err
				}

				eg.Go(func() error {
					return handleConn(conn)
				})
			}
		})

		return eg.Wait()
	})

	return cmd
}

func selectClusterFriendly(ctx context.Context, args []string) (*api.KubernetesCluster, error) {
	cluster, _, err := SelectRunningCluster(ctx, args)
	if errors.Is(err, ErrEmptyClusterList) {
		PrintCreateClusterMsg(ctx)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return cluster, nil
}
