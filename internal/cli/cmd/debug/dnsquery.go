// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/dns"
)

func newDnsQuery() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dnsquery",
		Short: "Queries a result with naming/dns.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			d := dns.Resolver{Timeout: 2 * time.Second, Nameservers: []string{"1.1.1.1:53"}}

			out := console.Stdout(ctx)
			for _, arg := range args {
				res, err := d.Lookup(arg)
				if err != nil {
					fmt.Fprintf(out, "%s: failed: %v\n", arg, err)
				} else {
					fmt.Fprintf(out, "%s: result: %s\n", arg, res)
				}
			}

			return nil
		}),
	}

	return cmd
}
