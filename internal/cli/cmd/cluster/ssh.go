package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnnet"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func newSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [cluster-id]",
		Short: "Start an SSH session.",
		Args:  cobra.MaximumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
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

		return dossh(ctx, cluster, nil)
	})

	return cmd
}

func dossh(ctx context.Context, cluster *api.KubernetesCluster, args []string) error {
	lst, err := fnnet.ListenPort(ctx, "127.0.0.1", 0, 0)
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

				peerConn, err := api.DialPort(ctx, cluster, 22)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
					return
				}

				go func() {
					_, _ = io.Copy(conn, peerConn)
				}()

				_, _ = io.Copy(peerConn, conn)
			}()
		}
	}()

	localPort := lst.Addr().(*net.TCPAddr).Port

	args = append(args,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "UpdateHostKeys no",
		"-p", fmt.Sprintf("%d", localPort), "root@127.0.0.1")

	if cluster.SshPrivateKey != nil {
		f, err := dirs.CreateUserTemp("ssh", "privatekey")
		if err != nil {
			return err
		}

		defer os.Remove(f.Name())

		if _, err := f.Write(cluster.SshPrivateKey); err != nil {
			return err
		}

		args = append(args, "-i", f.Name())
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	return localexec.RunInteractive(ctx, cmd)
}
