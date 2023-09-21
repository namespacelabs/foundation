package private

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	c "github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

func newInternalTerminalAttach() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Attach to an existing terminal.",
		Args:  cobra.NoArgs,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cli, err := containerd.New("/var/run/containerd/containerd.sock")
		if err != nil {
			return err
		}

		containers, err := cli.Containers(ctx, fmt.Sprintf("labels.%q==%s", "nsc/ctr-container-type", "terminal-source"))
		if err != nil {
			return err
		}

		if len(containers) == 0 {
			return errors.New("no terminal source registered")
		}

		target := containers[0]

		execArgs := []string{
			"exec", "-it",
		}

		if authSock := os.Getenv("SSH_AUTH_SOCK"); authSock != "" {
			execArgs = append(execArgs, "-e", "SSH_AUTH_SOCK="+authSock)
		}

		execArgs = append(execArgs, target.ID())

		stdin, err := c.ConsoleFromFile(os.Stdin)
		if err != nil {
			return err
		}

		cmd := exec.CommandContext(ctx, "nerdctl", execArgs...)

		cmd.Stdin = stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		done := console.EnterInputMode(ctx)
		defer done()

		if err := stdin.SetRaw(); err != nil {
			return err
		}

		defer stdin.Reset()

		return cmd.Run()
	})

	return cmd
}
