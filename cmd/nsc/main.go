// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ia "namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/cmd/sdk"
	"namespacelabs.dev/foundation/internal/cli/cmd/version"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/colors"
	ghenv "namespacelabs.dev/foundation/internal/github/env"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/std/protocol"
)

func main() {
	// Consider adding auto updates if we frequently change nsc.
	fncobra.DoMain("nsc", true, formatErr, func(root *cobra.Command) {
		api.SetupFlags("", root.PersistentFlags(), false)
		ia.SetupFlags(root.PersistentFlags())

		root.AddCommand(auth.NewAuthCmd())
		root.AddCommand(auth.NewLoginCmd()) // register `nsc login` as an alias for `nsc auth login`

		root.AddCommand(version.NewVersionCmd())

		root.AddCommand(cluster.NewBareClusterCmd(false))
		root.AddCommand(cluster.NewKubectlCmd())          // nsc kubectl
		root.AddCommand(cluster.NewKubeconfigCmd())       // nsc kubeconfig write
		root.AddCommand(cluster.NewBuildkitCmd())         // nsc buildkit builctl
		root.AddCommand(cluster.NewBuildCmd())            // nsc build
		root.AddCommand(cluster.NewDockerLoginCmd(false)) // nsc docker-login
		root.AddCommand(cluster.NewMetadataCmd())         // nsc metadata
		root.AddCommand(cluster.NewCreateCmd(false))      // nsc create
		root.AddCommand(cluster.NewListCmd())             // nsc list
		root.AddCommand(cluster.NewLogsCmd())             // nsc logs
		root.AddCommand(cluster.NewExposeCmd())           // nsc expose
		root.AddCommand(cluster.NewRunCmd())              // nsc run
		root.AddCommand(cluster.NewRunComposeCmd())       // nsc run-compose
		root.AddCommand(cluster.NewSshCmd())              // nsc ssh

		root.AddCommand(sdk.NewSdkCmd(true))

		fncobra.PushPreParse(root, func(ctx context.Context, args []string) error {
			api.Register()
			return nil
		})
	})
}

func formatErr(out io.Writer, style colors.Style, err error) {
	st, _ := unwrapStatus(err)

	var rid *protocol.RequestID
	for _, msg := range st.Proto().Details {
		decRid := &protocol.RequestID{}
		if msg.MessageIs(decRid) {
			if msg.UnmarshalTo(decRid) == nil {
				rid = decRid
			}
		}
	}

	if st.Code() == codes.Unknown {
		// Not a status error
		fncobra.DefaultErrorFormatter(out, style, err)
		return
	}

	ww := wordwrap.NewWriter(100)
	fmt.Fprintln(ww)
	fmt.Fprint(ww, style.ErrorHeader.Apply("Failed: "))

	switch st.Code() {
	case codes.PermissionDenied:
		fmt.Fprintf(ww, "it seems that's not allowed. We got: %s\n", st.Message())

		if rid != nil {
			fmt.Fprintln(ww)

			fmt.Fprint(ww, style.Comment.Apply("If this was unexpected, reach out to our team at "),
				style.CommentHighlight.Apply("support@namespacelabs.com"),
				style.Comment.Apply(" and mention request ID "),
				style.CommentHighlight.Apply(rid.Id),
			)
			fmt.Fprintln(ww)
		}

	case codes.Unauthenticated:
		fmt.Fprintf(ww, "no credentials found.\n")

		var cmd string
		switch {
		case ghenv.IsRunningInActions():
			cmd = "auth exchange-github-token"
		default:
			cmd = "login"
		}

		fmt.Fprintln(ww)
		fmt.Fprint(ww, style.Comment.Apply("Please run "),
			style.CommentHighlight.Apply(cmd),
			style.Comment.Apply("."))

	default:
		fmt.Fprintf(ww, "we got an error from our server: %s (%d)\n", st.Message(), st.Code())

		fmt.Fprintln(ww)
		fmt.Fprint(ww, style.Comment.Apply("This was unexpected. Please reach out to our team at "),
			style.CommentHighlight.Apply("support@namespacelabs.com"))

		if rid != nil {
			fmt.Fprint(ww,
				style.Comment.Apply(" and mention request ID "),
				style.CommentHighlight.Apply(rid.Id),
			)
		}

		fmt.Fprint(ww, style.Comment.Apply("."))
	}

	fmt.Fprintln(ww)
	_ = ww.Close()
	_, _ = out.Write(ww.Bytes())
}

func unwrapStatus(err error) (*status.Status, bool) {
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return unwrapStatus(unwrapped)
	}

	return status.FromError(err)
}
