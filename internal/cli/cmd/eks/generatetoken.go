// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthenticationv1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/aws/eks"
	"namespacelabs.dev/foundation/std/planning"
)

func newGenerateTokenCmd() *cobra.Command {
	var execCredential bool
	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "generate-token",
		Short: "Generates a EKS session token.",
		Args:  cobra.ExactArgs(1),
	}, func(ctx context.Context, env planning.Context, args []string) error {
		s, err := eks.NewSession(ctx, env.Configuration())
		if err != nil {
			return err
		}

		token, err := eks.ComputeBearerToken(ctx, s, args[0])
		if err != nil {
			return err
		}

		if execCredential {
			cred := &clientauthenticationv1.ExecCredential{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ExecCredential",
					APIVersion: "client.authentication.k8s.io/v1",
				},
				Status: &clientauthenticationv1.ExecCredentialStatus{
					ExpirationTimestamp: &metav1.Time{
						Time: token.Expiration,
					},
					Token: token.Token,
				},
			}
			w := json.NewEncoder(console.Stdout(ctx))
			return w.Encode(cred)
		} else {
			fmt.Fprintln(console.Stdout(ctx), token.Token)
		}
		return nil
	})

	cmd.Flags().BoolVar(&execCredential, "exec_credential", false, "Whether to output the token in the format expected by Kubernetes credential plugin system.")
	return cmd
}
