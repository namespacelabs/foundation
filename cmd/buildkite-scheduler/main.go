package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

var (
	rendezvouzEndpoint = pflag.String("rendezvous_endpoint", "rendezvous.namespace.so:5000", "namespace proxy endpoint")
	webhookSecret      = pflag.String("webhook_secret", "", "webhook secret")
	agentToken         = pflag.String("agent_token", "", "agent token")
)

func init() {
	api.SetupFlags("", pflag.CommandLine, false)
	api.Register()
}

func main() {
	fncobra.DoMain(fncobra.MainOpts{
		Name: "buildkite-scheduler",
		RegisterCommands: func(root *cobra.Command) {
			api.SetupFlags("", root.PersistentFlags(), false)
			root.RunE = func(cmd *cobra.Command, args []string) error {
				listener := StartProxyListener(cmd.Context(), *rendezvouzEndpoint, func(endpoint string) {
					fmt.Printf("Set webhook URL to http://%s/webhook\n", endpoint)
				})
				sched := scheduler{*agentToken}         //"117bfad10e5282d6aa5b5701e2bd28b6201e4e2b464cbceb1a"
				webhookSecret := []byte(*webhookSecret) //"a854f35d352eb8a26b3193066ae8b992"
				handler := webhookHandler(webhookSecret, sched.onWebHook)
				srv := &http.Server{
					Handler: handler,
					BaseContext: func(net.Listener) context.Context {
						return cmd.Context()
					},
				}
				return srv.Serve(listener)
			}
		},
	})
}
