// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/muesli/reflow/padding"
	"namespacelabs.dev/foundation/devworkflow/keyboard"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/schema/storage"
)

// Shows "Support services" and "Main services" in the terminal.
type StickyTextRenderer struct {
	ShowSupportServers bool
	plan               *storage.NetworkPlan
}

func NewStickyTextRenderer(name string, checkmark bool) *StickyTextRenderer {
	return &StickyTextRenderer{}
}

func (r *StickyTextRenderer) render(ctx context.Context) {
	content := ""
	if r.plan != nil {
		var out bytes.Buffer
		NetworkPlanToText(&out, r.plan, &NetworkPlanToTextOpts{
			Style:                 colors.WithColors,
			Checkmark:             true,
			IncludeSupportServers: r.ShowSupportServers})
		content = out.String()
	}
	console.SetStickyContent(ctx, "stack", content)
}

func (r *StickyTextRenderer) UpdatePlan(ctx context.Context, plan *storage.NetworkPlan) {
	r.plan = plan
	r.render(ctx)
}

func (r *StickyTextRenderer) SetShowSupportServers(ctx context.Context, showSupportServers bool) {
	r.ShowSupportServers = showSupportServers
	r.render(ctx)
}

type NetworkPlanToTextOpts struct {
	Style                 colors.Style
	Checkmark             bool
	IncludeSupportServers bool
}

func NetworkPlanToText(out io.Writer, r *storage.NetworkPlan, opts *NetworkPlanToTextOpts) {
	if r.LocalHostName == "" {
		fmt.Fprintf(out, "Services deployed:\n")
	} else {
		fmt.Fprintf(out, "Development mode, services forwarded to %s.\n", opts.Style.LessRelevant.Apply("localhost"))
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

	if opts.IncludeSupportServers && len(supportServices) > 0 {
		fmt.Fprintf(out, "\n %s\n\n", opts.Style.Comment.Apply("Support services:"))

		renderNotFocusedEndpointsText(out, opts.Style, supportServices, opts.Checkmark)
	}

	if len(mainServices) > 0 {
		fmt.Fprint(out, "\n Main services:\n")

		renderFocusedEndpointsText(out, opts.Style, mainServices, opts.Checkmark)
	}

	if len(r.Endpoint) == 0 {
		fmt.Fprintf(out, "   %s\n", opts.Style.LessRelevant.Apply("No services exported"))
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
		label := style.Comment.Apply(renderLabel(entry.Label))
		url := style.Comment.Apply(entry.AccessCmd[0].Cmd)

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

type SupportServicesKeybinding struct {
	renderer *StickyTextRenderer
}

func NewSupportServicesKeybinding(renderer *StickyTextRenderer) *SupportServicesKeybinding {
	return &SupportServicesKeybinding{renderer: renderer}
}

func (k SupportServicesKeybinding) Key() string { return "s" }

func (k SupportServicesKeybinding) Label(enabled bool) string {
	if !enabled {
		return "show support servers"
	}
	return "hide support servers " // Additional space at the end for a better allignment.
}

func (k SupportServicesKeybinding) Handle(ctx context.Context, ch chan keyboard.Event, control chan<- keyboard.Control) {
	for event := range ch {
		switch event.Operation {
		case keyboard.OpSet:
			k.renderer.SetShowSupportServers(ctx, event.Enabled)

			c := keyboard.Control{Operation: keyboard.ControlAck}
			c.AckEvent.HandlerID = event.HandlerID
			c.AckEvent.EventID = event.EventID

			control <- c
		}
	}
}
