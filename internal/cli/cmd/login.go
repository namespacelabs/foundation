// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
)

const loginUrl = "https://signin.prod.namespacelabs.nscloud.dev/login"

func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to use Foundation services (DNS and SSL management, etc).",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			out := console.Stdout(ctx)
			fmt.Fprintln(out, "Open the following URL in your browser, and then copy-paste the resulting code:")
			fmt.Fprintln(out)
			fmt.Fprintln(out, "  ", loginUrl)
			fmt.Fprintln(out)

			done := console.EnterInputMode(ctx, "Code: ")
			defer done()

			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				username, err := fnapi.StoreUser(ctx, scanner.Text())
				if err != nil {
					return err
				}

				fmt.Println()
				fmt.Printf("Hi %s, you are now logged in, have a nice day.\n", username)
			}

			return nil
		}),
	}

	return cmd
}
