package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnnet"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newPortForwardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward [cluster-id]",
		Short: "Opens a local port which connects to the cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	port := cmd.Flags().Int("target_port", 0, "Which port to forward to.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *port == 0 {
			return fnerrors.New("--target_port is required")
		}

		cluster, err := selectCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusteList) {
				printCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		return portForward(ctx, cluster, *port)
	})

	return cmd
}

func portForward(ctx context.Context, cluster *api.KubernetesCluster, targetPort int) error {
	lst, err := fnnet.ListenPort(ctx, "127.0.0.1", 0, targetPort)
	if err != nil {
		return err
	}

	localPort := lst.Addr().(*net.TCPAddr).Port
	fmt.Fprintf(console.Stdout(ctx), "Listening on 127.0.0.1:%d\n", localPort)

	for {
		conn, err := lst.Accept()
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "New connection from %v\n", conn.RemoteAddr())

		go func() {
			defer conn.Close()

			proxyConn, err := api.DialPort(ctx, cluster, targetPort)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
				return
			}

			go func() {
				_, _ = io.Copy(conn, proxyConn)
			}()

			_, _ = io.Copy(proxyConn, conn)
		}()
	}
}
