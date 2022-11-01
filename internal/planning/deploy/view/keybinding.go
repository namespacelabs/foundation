// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package view

import (
	"bytes"
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/planning/render"
	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/schema/storage"
)

type NetworkPlanKeybinding struct {
	name               string
	showSupportServers bool
}

func NewNetworkPlanKeybinding(name string) *NetworkPlanKeybinding {
	return &NetworkPlanKeybinding{
		name:               name,
		showSupportServers: false,
	}
}

func (k *NetworkPlanKeybinding) Key() string { return "s" }

func (k *NetworkPlanKeybinding) Label() string {
	if !k.showSupportServers {
		return "show support servers"
	}
	return "hide support servers"
}

func (k *NetworkPlanKeybinding) Handle(ctx context.Context, ch chan keyboard.Event, control chan<- keyboard.Control) {
	var networkPlan *storage.NetworkPlan

	for event := range ch {
		switch event.Operation {
		case keyboard.OpToggle:
			k.showSupportServers = !k.showSupportServers

			k.renderStickyNetworkPlan(ctx, networkPlan)

			c := keyboard.Control{Operation: keyboard.ControlAck}
			c.AckEvent.HandlerID = event.HandlerID
			c.AckEvent.EventID = event.EventID

			control <- c

		case keyboard.OpStackUpdate:
			networkPlan = event.StackUpdate.NetworkPlan

			k.renderStickyNetworkPlan(ctx, networkPlan)
		}
	}
}

func (k NetworkPlanKeybinding) renderStickyNetworkPlan(ctx context.Context, plan *storage.NetworkPlan) {
	content := ""
	if plan != nil {
		summary := render.NetworkPlanToSummary(plan)
		var out bytes.Buffer
		NetworkPlanToText(&out, summary, &NetworkPlanToTextOpts{
			Style:                 colors.WithColors,
			Checkmark:             true,
			IncludeSupportServers: k.showSupportServers,
		})
		content = out.String()
	}

	if plan != nil && plan.IsDeploymentFinished() {
		fmt.Fprintln(console.TypedOutput(ctx, "network plan", common.CatOutputUs), content)
		console.SetStickyContent(ctx, k.name, "")
	} else {
		console.SetStickyContent(ctx, k.name, content)
	}
}
