// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"fmt"
	"io"
	"strings"

	"github.com/muesli/reflow/padding"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/schema/storage"
)

func RenderText(out io.Writer, style colors.Style, r *storage.NetworkPlan, checkmark bool, localHostname string) {
	if localHostname == "" {
		fmt.Fprintf(out, " Services deployed:\n\n")
	} else {
		fmt.Fprintf(out, " Services forwarded to %s", style.LessRelevant.Apply("localhost"))
		if r.InternalCount > 0 {
			fmt.Fprintf(out, " (+%d internal)", r.InternalCount)
		}
		fmt.Fprintf(out, ":\n\n")
	}

	var longestLabel, longestUrl uint
	for _, entry := range r.Endpoint {
		if l := uint(len(renderLabel(entry.Label))); l > longestLabel {
			longestLabel = l
		}

		if l := uint(len(entry.Url)); l > longestUrl {
			longestUrl = l
		}
	}

	for k, entry := range r.Endpoint {
		label := renderLabel(entry.Label)
		url := entry.Url

		if entry.Focus {
			label = style.Highlight.Apply(label)
		} else {
			label = style.Header.Apply(label)
		}

		if !entry.Focus {
			url = style.Header.Apply(url)
		}

		if entry.Focus {
			if k > 0 && !r.Endpoint[k-1].Focus {
				fmt.Fprintln(out)
			}
		}

		fmt.Fprintf(out, " %s%s  %s%s\n", checkLabel(style, checkmark, entry.Focus, entry.LocalPort),
			padding.String(label, longestLabel), padding.String(url, longestUrl+1),
			comment(style, entry.EndpointOwner))
	}

	if len(r.Endpoint) == 0 {
		fmt.Fprintf(out, "   %s\n", style.LessRelevant.Apply("(none)"))
	}

	renderIngressText(out, style, r, checkmark, "Ingress endpoints forwarded to your workstation")
	renderIngressBlockText(out, style, "Ingress configured", r.NonLocalManaged)
	renderIngressBlockText(out, style, "Ingress configured, but not managed", r.NonLocalNonManaged)
}

func renderIngressText(out io.Writer, style colors.Style, r *storage.NetworkPlan, checkmark bool, label string) {
	if len(r.Ingress) == 0 {
		return
	}

	fmt.Fprintf(out, "\n %s:\n\n", label)

	for _, ingress := range r.Ingress {
		fmt.Fprintf(out, " %s%s%s%s%s%s\n", checkLabel(style, checkmark, true, ingress.LocalPort),
			ingress.Schema, ingress.Fqdn, ingress.PortLabel, ingress.Command, comment(style, ingress.Comment))
	}
}

func renderIngressBlockText(out io.Writer, style colors.Style, label string, fragments []*storage.NetworkPlan_Ingress) {
	if len(fragments) == 0 {
		return
	}

	fmt.Fprintf(out, "\n %s:\n\n", label)

	labels := make([]string, len(fragments))
	comments := make([]string, len(fragments))

	var longestLabel uint
	for k, n := range fragments {
		labels[k] = fmt.Sprintf("%s%s%s%s", n.Schema, n.Fqdn, n.PortLabel, n.Command)
		comments[k] = n.Comment

		if x := uint(len(labels[k])); x > longestLabel {
			longestLabel = x
		}
	}

	for k, n := range fragments {
		fmt.Fprintf(out, " %s%s %s%s\n", checkbox(style, true, false),
			padding.String(labels[k], longestLabel),
			comment(style, strings.Join(n.PackageOwner, ", ")), comment(style, comments[k]))
	}
}

func checkLabel(style colors.Style, b, isFocus bool, port uint32) string {
	return checkbox(style, !b || port > 0, !isFocus)
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

func renderLabel(lbl *storage.NetworkPlan_Label) string {
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
