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
		fmt.Fprintf(out, "Development mode, services forwarded to %s.\n\n", style.LessRelevant.Apply("localhost"))
	}

	supportServices := []*storage.NetworkPlan_Endpoint{}
	mainServices := []*storage.NetworkPlan_Endpoint{}
	for _, entry := range r.Endpoint {
		if entry.Focus {
			mainServices = append(mainServices, entry)
		} else {
			supportServices = append(supportServices, entry)
		}
	}

	if len(supportServices) > 0 {
		fmt.Fprint(out, style.Header.Apply(" Support services:\n\n"))

		renderNotFocusedEndpointsText(out, style, supportServices, checkmark)
	}

	if len(mainServices) > 0 {
		fmt.Fprint(out, "\n Main services:\n")

		renderFocusedEndpointsText(out, style, mainServices, checkmark)
	}

	if len(r.Endpoint) == 0 {
		fmt.Fprintf(out, "   %s\n", style.LessRelevant.Apply("No services exported"))
	}
}

func renderNotFocusedEndpointsText(out io.Writer, style colors.Style, services []*storage.NetworkPlan_Endpoint, checkmark bool) {
	var longestLabel, longestUrl uint
	for _, entry := range services {
		if l := uint(len(renderLabel(entry.Label))); l > longestLabel {
			longestLabel = l
		}

		for _, cmd := range entry.AccessCmd {
			if l := uint(len(cmd.Cmd)); l > longestUrl {
				longestUrl = l
			}
		}
	}

	for _, entry := range services {
		label := style.Header.Apply(renderLabel(entry.Label))
		url := style.Header.Apply(entry.AccessCmd[0].Cmd)

		fmt.Fprintf(out, " %s%s  %s%s\n",
			checkLabel(style, checkmark, entry.Focus, entry.IsPortForwarded),
			padding.String(label, longestLabel),
			padding.String(url, longestUrl+1),
			comment(style, entry.EndpointOwner))
	}
}

func renderFocusedEndpointsText(out io.Writer, style colors.Style, services []*storage.NetworkPlan_Endpoint, checkmark bool) {
	hasNotManagedDomains := false

	for _, entry := range services {
		fmt.Fprintf(out, "\n %s%s\n",
			checkLabel(style, checkmark, entry.Focus, entry.IsPortForwarded),
			style.Highlight.Apply(renderLabel(entry.Label)))
		for _, cmd := range entry.AccessCmd {
			notManagedHint := "  "
			if !cmd.IsManaged {
				notManagedHint = style.LessRelevant.Apply("* ")
				hasNotManagedDomains = true
			}
			fmt.Fprintf(out, "    %s%s\n", notManagedHint, style.Comment.Apply(cmd.Cmd))
		}
	}

	if hasNotManagedDomains {
		fmt.Fprintf(out, "\n %s\n     %s\n",
			style.LessRelevant.Apply("(*) The ingress has been configured to support this domain name, but its DNS records are not managed by Namespace."),
			style.LessRelevant.Apply("See https://docs.namespace.so/reference/managed-domains/ for more details."))
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
