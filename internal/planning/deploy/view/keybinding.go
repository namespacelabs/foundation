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
	Name string
}

func (k NetworkPlanKeybinding) Key() string { return "s" }

func (k NetworkPlanKeybinding) States() []keyboard.HandlerState {
	return []keyboard.HandlerState{
		{State: "hidden", Label: "show support servers"},
		{State: "shown", Label: "hide support servers"},
	}
}

func (k NetworkPlanKeybinding) Handle(ctx context.Context, ch chan keyboard.Event, control chan<- keyboard.Control) {
	defer close(control)

	showSupportServers := false
	var networkPlan *storage.NetworkPlan

	for event := range ch {
		switch event.Operation {
		case keyboard.OpSet:
			showSupportServers := event.CurrentState == "shown"

			k.renderStickyNetworkPlan(ctx, networkPlan, showSupportServers)

			c := keyboard.Control{}
			c.AckEvent.EventID = event.EventID

			control <- c

		case keyboard.OpStackUpdate:
			networkPlan = event.StackUpdate.NetworkPlan

			k.renderStickyNetworkPlan(ctx, networkPlan, showSupportServers)

			set := event.StackUpdate.Deployed
			control <- keyboard.Control{
				SetEnabled: &set,
			}
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

	if plan != nil && plan.IsDeploymentFinished() {
		fmt.Fprintln(console.TypedOutput(ctx, "network plan", common.CatOutputUs), content)
		console.SetStickyContent(ctx, k.Name, "")
	} else {
		console.SetStickyContent(ctx, k.Name, content)
	}
}
