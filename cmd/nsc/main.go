// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	iamv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/iam/v1beta"
	"connectrpc.com/connect"
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

func formatPermission(perm *iamv1beta.Access) string {
	if perm.GetResourceId() != "" {
		return fmt.Sprintf("%s:%s on %s", perm.GetResourceType(), perm.GetAction(), perm.GetResourceId())
	}

	return fmt.Sprintf("%s:%s", perm.GetResourceType(), perm.GetAction())
}

type rpcError struct {
	code    codes.Code
	message string
	rid     *protocol.RequestID
	details []proto.Message
}

func rpcErrorFromConnect(connectErr *connect.Error) *rpcError {
	re := &rpcError{
		code:    codes.Code(connectErr.Code()),
		message: connectErr.Message(),
	}

	for _, detail := range connectErr.Details() {
		value, err := detail.Value()
		if err != nil {
			continue
		}
		re.details = append(re.details, value)
		if rid, ok := value.(*protocol.RequestID); ok {
			re.rid = rid
		}
	}

	return re
}

func rpcErrorFromStatus(st *status.Status) *rpcError {
	re := &rpcError{
		code:    st.Code(),
		message: st.Message(),
	}

	for _, any := range st.Proto().Details {
		msg, err := any.UnmarshalNew()
		if err != nil {
			continue
		}
		re.details = append(re.details, msg)
		if rid, ok := msg.(*protocol.RequestID); ok {
			re.rid = rid
		}
	}

	return re
}

func (re *rpcError) findDetail(target proto.Message) bool {
	for _, d := range re.details {
		if proto.MessageName(d) == proto.MessageName(target) {
			proto.Merge(target, d)
			return true
		}
	}
	return false
}

func formatErr(out io.Writer, style colors.Style, err error) {
	var re *rpcError

	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		re = rpcErrorFromConnect(connectErr)
	} else {
		st, _ := unwrapStatus(err)
		if st.Code() == codes.Unknown {
			fncobra.DefaultErrorFormatter(out, style, err)
			return
		}
		re = rpcErrorFromStatus(st)
	}

	if re == nil {
		fncobra.DefaultErrorFormatter(out, style, err)
		return
	}

	ww := wordwrap.NewWriter(100)
	fmt.Fprintln(ww)
	fmt.Fprint(ww, style.ErrorHeader.Apply("Failed: "))

	msg := re.message
	var userMsg v1.UserMessage
	if re.findDetail(&userMsg) && userMsg.Message != "" {
		msg = userMsg.Message
	}

	switch re.code {
	case codes.PermissionDenied:
		var permDenied iamv1beta.PermissionDeniedError
		re.findDetail(&permDenied)

		switch len(permDenied.GetMissingAccessPermissions()) {
		case 0:
			fmt.Fprintf(ww, "it seems that's not allowed. We got: %s\n", msg)

		case 1:
			permStr := formatPermission(permDenied.MissingAccessPermissions[0])
			fmt.Fprintf(ww, "that's not allowed. You're missing the permission %s.\n", style.CommentHighlight.Apply(permStr))

		default:
			fmt.Fprintf(ww, "that's not allowed. You're missing the following permissions:\n")
			for _, perm := range permDenied.MissingAccessPermissions {
				permStr := formatPermission(perm)
				fmt.Fprintf(ww, "  • %s\n", style.CommentHighlight.Apply(permStr))
			}
		}

		if re.rid != nil {
			fmt.Fprintln(ww)
			fmt.Fprint(ww, style.Comment.Apply("If this was unexpected, reach out to our team at "),
				style.CommentHighlight.Apply("support@namespace.so"),
				style.Comment.Apply(" and mention request ID "),
				style.CommentHighlight.Apply(re.rid.Id),
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
		var envNotExist v1.EnvironmentDoesntExist
		if re.findDetail(&envNotExist) {
			fmt.Fprintf(ww, "%q does not exist.", envNotExist.ClusterId)
		} else {
			generic(ww, style, re.code, msg, re.rid)
		}

	case codes.FailedPrecondition:
		var envDestroyed v1.EnvironmentDestroyed
		if re.findDetail(&envDestroyed) {
			fmt.Fprintf(ww, "%q is no longer running.", envDestroyed.ClusterId)
		} else {
			generic(ww, style, re.code, msg, re.rid)
		}

	case codes.ResourceExhausted:
		fmt.Fprintf(ww, "ran out of capacity: %s%s.\n", msg, appendRid(re.rid))

	default:
		generic(ww, style, re.code, msg, re.rid)
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
