// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	ia "namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/cmd/sdk"
	"namespacelabs.dev/foundation/internal/cli/cmd/version"
	"namespacelabs.dev/foundation/internal/cli/cmd/workspace"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	ghenv "namespacelabs.dev/foundation/internal/github/env"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/text/timefmt"
	v1 "namespacelabs.dev/foundation/public/nscloud/proto/v1"
	"namespacelabs.dev/foundation/std/protocol"
	"namespacelabs.dev/foundation/std/tasks"
)

func main() {
	// Consider adding auto updates if we frequently change nsc.
	fncobra.DoMain(fncobra.MainOpts{
		Name:                 "nsc",
		AutoUpdate:           true,
		ConsoleInhibitReport: true,
		FormatErr:            formatErr,
		ConsoleRenderer:      renderLine,
		RegisterCommands: func(root *cobra.Command) {
			api.SetupFlags("", root.PersistentFlags(), false)
			ia.SetupFlags(root.PersistentFlags())

			root.AddCommand(auth.NewAuthCmd())
			root.AddCommand(auth.NewLoginCmd()) // register `nsc login` as an alias for `nsc auth login`

			root.AddCommand(version.NewVersionCmd())

			root.AddCommand(cluster.NewBareClusterCmd(false))
			root.AddCommand(cluster.NewKubectlCmd())    // nsc kubectl
			root.AddCommand(cluster.NewKubeconfigCmd()) // nsc kubeconfig write
			root.AddCommand(cluster.NewBuildkitCmd())   // nsc buildkit
			root.AddCommand(cluster.NewBuildCmd())      // nsc build
			root.AddCommand(cluster.NewMetadataCmd())   // nsc metadata
			root.AddCommand(cluster.NewCreateCmd())     // nsc create
			root.AddCommand(cluster.NewListCmd())       // nsc list
			root.AddCommand(cluster.NewLogsCmd())       // nsc logs
			root.AddCommand(cluster.NewExposeCmd())     // nsc expose
			root.AddCommand(cluster.NewRunCmd())        // nsc run
			root.AddCommand(cluster.NewRunComposeCmd()) // nsc run-compose
			root.AddCommand(cluster.NewSshCmd())        // nsc ssh
			root.AddCommand(cluster.NewDockerCmd())     // nsc docker
			root.AddCommand(cluster.NewDescribeCmd())   // nsc describe
			root.AddCommand(cluster.NewExecScoped())    // nsc exec-scoped

			root.AddCommand(sdk.NewSdkCmd(true))

			root.AddCommand(workspace.NewWorkspaceCmd()) // nsc workspace
		},
	})
}

var progressRunes = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
var runeDur = int64(1000 / len(progressRunes))

func renderLine(w io.Writer, style colors.Style, t time.Time, li consolesink.Renderable) string {
	data := li.Data

	dur := t.Sub(li.Data.Started)

	progressRune := progressRunes[(dur.Milliseconds()%1000)/runeDur]

	fmt.Fprintf(w, "%s ", style.LogArgument.Apply(progressRune))

	name := data.HumanReadable
	if name == "" {
		name = data.Name
	}

	fmt.Fprint(w, name)

	if progress := li.Progress; progress != nil && data.State == tasks.ActionRunning {
		if p := progress.FormatProgress(); p != "" {
			fmt.Fprint(w, " ", style.Progress.Apply(p))
		}
	}

	if dur > 3*time.Second {
		return " (" + timefmt.Seconds(dur) + ") "
	}

	return ""
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

	msg := st.Message()
	if x, ok := hasDetail(st, &v1.UserMessage{}); ok && x.Message != "" {
		msg = x.Message
	}

	switch st.Code() {
	case codes.PermissionDenied:
		fmt.Fprintf(ww, "it seems that's not allowed. We got: %s\n", msg)

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
		fmt.Fprintf(ww, "no credentials found. We got: %s\n\n", msg)

		var cmd string
		switch {
		case ghenv.IsRunningInActions():
			cmd = "auth exchange-github-token"
		default:
			cmd = "login"
		}

		fmt.Fprintln(ww)
		fmt.Fprint(ww, style.Comment.Apply("Please run "),
			style.CommentHighlight.Apply("nsc "+cmd),
			style.Comment.Apply("."))

	case codes.NotFound:
		if x, ok := hasDetail(st, &v1.EnvironmentDoesntExist{}); ok {
			fmt.Fprintf(ww, "%q does not exist.", x.ClusterId)
		} else {
			generic(ww, style, st.Code(), msg, rid)
		}

	case codes.FailedPrecondition:
		if x, ok := hasDetail(st, &v1.EnvironmentDestroyed{}); ok {
			fmt.Fprintf(ww, "%q is no longer running.", x.ClusterId)
		} else {
			generic(ww, style, st.Code(), msg, rid)
		}

	default:
		generic(ww, style, st.Code(), msg, rid)
	}

	fmt.Fprintln(ww)
	_ = ww.Close()
	_, _ = out.Write(ww.Bytes())
}

func hasDetail[Msg proto.Message](st *status.Status, detail Msg) (Msg, bool) {
	for _, x := range st.Proto().Details {
		if x.MessageIs(detail) {
			if x.UnmarshalTo(detail) == nil {
				return detail, true
			}
			return detail, false
		}
	}

	return detail, false
}

func generic(ww io.Writer, style colors.Style, code codes.Code, msg string, rid *protocol.RequestID) {
	fmt.Fprintf(ww, "we got an error from our server: %s (%s)\n", msg, code)

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

func unwrapStatus(err error) (*status.Status, bool) {
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return unwrapStatus(unwrapped)
	}

	return status.FromError(err)
}
