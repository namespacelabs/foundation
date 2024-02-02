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
	nsccmd "namespacelabs.dev/foundation/cmd/nsc/cmd"
	ia "namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	ghenv "namespacelabs.dev/foundation/internal/github/env"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
	"namespacelabs.dev/foundation/internal/text/timefmt"
	v1 "namespacelabs.dev/foundation/public/nscloud/proto/v1"
	"namespacelabs.dev/foundation/std/protocol"
	"namespacelabs.dev/foundation/std/tasks"
)

func main() {
	// Consider adding auto updates if we frequently change nsc.
	fncobra.DoMain(fncobra.MainOpts{
		Name:                 "nsc",
		NotifyOnNewVersion:   true,
		ConsoleInhibitReport: true,
		FormatErr:            formatErr,
		ConsoleRenderer:      renderLine,
		RegisterCommands: func(root *cobra.Command) {
			endpoint.SetupFlags("", root.PersistentFlags(), false)
			ia.SetupFlags(root.PersistentFlags())
			nsccmd.RegisterCommands(root)
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
				style.CommentHighlight.Apply("support@namespace.so"),
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

	case codes.ResourceExhausted:
		fmt.Fprintf(ww, "ran out of capacity: %s%s.\n", msg, appendRid(rid))

	default:
		generic(ww, style, st.Code(), msg, rid)
	}

	fmt.Fprintln(ww)
	_ = ww.Close()
	_, _ = out.Write(ww.Bytes())
}

func appendRid(rid *protocol.RequestID) string {
	if rid == nil {
		return ""
	}

	return fmt.Sprintf(" (request id: %s)", rid.GetId())
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
	fmt.Fprintf(ww, "%s (%s).\n", msg, code)

	fmt.Fprintln(ww)
	fmt.Fprint(ww, style.Comment.Apply("This was unexpected. Please reach out to our team at "),
		style.CommentHighlight.Apply("support@namespace.so"))

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
