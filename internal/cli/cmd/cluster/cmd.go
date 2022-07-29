// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cluster

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/chisel/share/cnet"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/localexec"
)

func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "cluster",
		Short:  "Cluster-related activities (internal only).",
		Hidden: true,
	}

	cmd.AddCommand(newSshCmd())

	return cmd
}

func newSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Start an SSH session.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusterId := args[0]

		lst, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}

		go func() {
			for {
				conn, err := lst.Accept()
				if err != nil {
					return
				}

				go func() {
					defer conn.Close()

					d := websocket.Dialer{
						HandshakeTimeout: 15 * time.Second,
					}

					serverUrl := fmt.Sprintf("ws://ssh-%s.a.nscluster.cloud/proxy", clusterId)
					wsConn, _, err := d.DialContext(ctx, serverUrl, nil)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
						return
					}

					proxyConn := cnet.NewWebSocketConn(wsConn)

					go func() {
						_, _ = io.Copy(conn, proxyConn)
					}()

					_, _ = io.Copy(proxyConn, conn)
				}()
			}
		}()

		localPort := lst.Addr().(*net.TCPAddr).Port

		sshArgs := args[1:]
		sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", localPort), "root@127.0.0.1")

		cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
		return localexec.RunInteractive(ctx, cmd)
	})

	return cmd
}
