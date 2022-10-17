// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package view

import (
	"bytes"
	"context"

	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/planning/deploy/render"
	"namespacelabs.dev/foundation/schema/storage"
)

type NetworkPlanKeybinding struct {
	name string
}

func NewNetworkPlanKeybinding(name string) *NetworkPlanKeybinding {
	return &NetworkPlanKeybinding{
		name: name,
	}
}

func (k NetworkPlanKeybinding) Key() string { return "s" }

func (k NetworkPlanKeybinding) Label(enabled bool) string {
	if !enabled {
		return "show support servers"
	}
	return "hide support servers"
}

func (k NetworkPlanKeybinding) Handle(ctx context.Context, ch chan keyboard.Event, control chan<- keyboard.Control) {
	showSupportServers := false
	var networkPlan *storage.NetworkPlan

	for event := range ch {
		switch event.Operation {
		case keyboard.OpSet:
			showSupportServers := event.Enabled

			k.renderStickyNetworkPlan(ctx, networkPlan, showSupportServers)

			c := keyboard.Control{Operation: keyboard.ControlAck}
			c.AckEvent.HandlerID = event.HandlerID
			c.AckEvent.EventID = event.EventID

			control <- c

		case keyboard.OpStackUpdate:
			networkPlan = event.StackUpdate.NetworkPlan

			k.renderStickyNetworkPlan(ctx, networkPlan, showSupportServers)
		}
	}
}

func (k NetworkPlanKeybinding) renderStickyNetworkPlan(ctx context.Context, plan *storage.NetworkPlan, showSupportServers bool) {
	content := ""
	if plan != nil {
		summary := render.NetworkPlanToSummary(plan)
		var out bytes.Buffer
		NetworkPlanToText(&out, summary, &NetworkPlanToTextOpts{
			Style:                 colors.WithColors,
			Checkmark:             true,
			IncludeSupportServers: showSupportServers,
		})
		content = out.String()
	}
	console.SetStickyContent(ctx, k.name, content)
}
