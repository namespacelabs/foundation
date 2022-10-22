// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/kr/text"
	"github.com/muesli/reflow/padding"
	"namespacelabs.dev/foundation/framework/planning/render"
	"namespacelabs.dev/foundation/internal/console/colors"
)

type NetworkPlanToTextOpts struct {
	Style                 colors.Style
	Checkmark             bool
	IncludeSupportServers bool
}

func NetworkPlanToText(out io.Writer, r *render.NetworkPlanSummary, opts *NetworkPlanToTextOpts) {
	if r.LocalHostname == "" {
		fmt.Fprintf(out, " Services deployed:\n")
	} else {
		fmt.Fprintf(out, " Development mode, services forwarded to %s:\n", r.LocalHostname)
	}

	if opts.IncludeSupportServers && len(r.SupportServices) > 0 {
		fmt.Fprintf(out, "\n %s\n\n", opts.Style.Comment.Apply("Support services:"))

		renderNotFocusedEndpointsText(indented(out), opts.Style, r.SupportServices, opts.Checkmark)
	}

	if len(r.FocusedServices) > 0 {
		if opts.IncludeSupportServers {
			fmt.Fprint(out, "\n Main services:\n")
		}

		renderFocusedEndpointsText(indented(out), opts.Style, r.FocusedServices, opts.Checkmark)
	}

	if len(r.FocusedServices) == 0 && len(r.SupportServices) == 0 {
		fmt.Fprintf(out, "\n   %s\n", opts.Style.LessRelevant.Apply("No services exported"))
	}
}

func indented(out io.Writer) io.Writer {
	return text.NewIndentWriter(out, []byte(" "))
}

func renderNotFocusedEndpointsText(out io.Writer, style colors.Style, services []*render.NetworkPlanSummary_Service, checkmark bool) {
	var longestLabel, longestCmd uint
	for _, entry := range services {
		if l := uint(len(renderLabel(entry.Label))); l > longestLabel {
			longestLabel = l
		}

		if l := uint(len(entry.AccessCmd[0].Cmd)); l > longestCmd {
			longestCmd = l
		}
	}

	for _, entry := range services {
		label := style.Comment.Apply(renderLabel(entry.Label))
		url := style.Comment.Apply(entry.AccessCmd[0].Cmd)

		fmt.Fprintf(out, "%s%s  %s%s\n",
			checkLabel(style, checkmark, false /* isFocus */, entry.LocalPort > 0),
			padding.String(label, longestLabel),
			padding.String(url, longestCmd+1),
			comment(style, entry.PackageName))
	}
}

func renderFocusedEndpointsText(out io.Writer, style colors.Style, services []*render.NetworkPlanSummary_Service, checkmark bool) {
	hasNotManagedDomains := false

	for _, entry := range services {
		fmt.Fprintf(out, "\n%s%s\n",
			checkLabel(style, checkmark, true /* isFocus */, entry.LocalPort > 0),
			style.Highlight.Apply(renderLabel(entry.Label)))
		for _, cmd := range entry.AccessCmd {
			notManagedHint := "  "
			if !cmd.IsManaged {
				notManagedHint = style.LessRelevant.Apply("* ")
				hasNotManagedDomains = true
			}
			fmt.Fprintf(out, "   %s%s\n", notManagedHint, style.Comment.Apply(cmd.Cmd))
		}
	}

	if hasNotManagedDomains {
		fmt.Fprintf(out, "\n %s\n     %s\n",
			style.Comment.Apply("(*) The ingress has been configured to support this domain name, but its DNS records are not managed by Namespace."),
			style.Comment.Apply("See https://docs.namespace.so/reference/managed-domains/ for more details."))
	}
}

func checkLabel(style colors.Style, b, isFocus bool, isPortForwarded bool) string {
	return checkbox(style, !b || isPortForwarded, !isFocus)
}

func checkbox(style colors.Style, on, notFocus bool) string {
	x := " [ ] "
	if on {
		x = " [✓] "
	}
	if notFocus {
		return style.Header.Apply(x)
	}
	return x
}

func comment(style colors.Style, str string) string {
	if str == "" {
		return ""
	}
	return style.Comment.Apply(" # " + str)
}

func renderLabel(lbl *render.Label) string {
	if lbl.ServiceProto != "" {
		return compressProtoTypename(lbl.ServiceProto)
	}

	return lbl.Label
}

func compressProtoTypename(t string) string {
	if len(t) < 40 {
		return t
	}
	parts := strings.Split(t, ".")
	for k := 0; k < len(parts)-1; k++ {
		parts[k] = string(parts[k][0])
	}
	return strings.Join(parts, ".")
}
