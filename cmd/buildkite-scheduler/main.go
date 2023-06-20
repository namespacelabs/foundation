package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

var (
	rendezvouzEndpoint = pflag.String("rendezvous_endpoint", "rendezvous.namespace.so:5000", "namespace proxy endpoint")
	webhookSecret      = pflag.String("webhook_secret", "", "webhook secret")
	agentToken         = pflag.String("agent_token", "", "agent token")
	apiToken           = pflag.String("api_token", "", "api token")
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
				eg, ctx := errgroup.WithContext(cmd.Context())

				sched := newScheduler(*agentToken, *apiToken)
				eg.Go(func() error {
					return sched.runWorker(ctx)
				})

				eg.Go(func() error {
					return sched.runPoller(ctx)
				})

				eg.Go(func() error {
					listener := StartProxyListener(ctx, *rendezvouzEndpoint, func(endpoint string) {
						fmt.Printf("Set webhook URL to http://%s/webhook\n", endpoint)
					})
					webhookSecret := []byte(*webhookSecret)
					handler := webhookHandler(webhookSecret, sched.onWebHook)
					srv := &http.Server{
						Handler:     handler,
						BaseContext: func(net.Listener) context.Context { return ctx },
					}
					return srv.Serve(listener)
				})

				return eg.Wait()
			}
		},
	})
}
