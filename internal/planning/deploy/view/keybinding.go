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
		{State: "hidden", Label: "show main services"},
		{State: "show", Label: "show all services"},
		{State: "show-all", Label: "hide services"},
	}
}

func (k NetworkPlanKeybinding) Handle(ctx context.Context, ch chan keyboard.Event, control chan<- keyboard.Control) {
	defer close(control)

	currentState := "hidden"

	var networkPlan *storage.NetworkPlan
	var deploymentRevision uint64

	for event := range ch {
		switch event.Operation {
		case keyboard.OpSet:
			currentState = event.CurrentState

		case keyboard.OpStackUpdate:
			networkPlan = event.StackUpdate.NetworkPlan
		}

		if currentState != "hidden" {
			console.SetStickyContent(ctx, k.Name, renderStickyNetworkPlan(networkPlan, currentState == "show-all"))
		} else {
			console.SetStickyContent(ctx, k.Name, "")

			if event.Operation == keyboard.OpStackUpdate && event.StackUpdate.Deployed && event.StackUpdate.DeployedRevision > deploymentRevision && networkPlan != nil && !networkPlan.Incomplete {
				deploymentRevision = event.StackUpdate.DeployedRevision
				if out := renderStickyNetworkPlan(networkPlan, false); out != "" {
					fmt.Fprintln(console.TypedOutput(ctx, "network plan", common.CatOutputUs), out)
				}
			}
		}

		switch event.Operation {
		case keyboard.OpSet:
			c := keyboard.Control{}
			c.AckEvent.EventID = event.EventID
			control <- c

		case keyboard.OpStackUpdate:
			set := event.StackUpdate.Deployed
			control <- keyboard.Control{
				SetEnabled: &set,
			}
		}
	}
}

func renderStickyNetworkPlan(plan *storage.NetworkPlan, showSupportServers bool) string {
	if plan == nil {
		return ""
	}

	summary := render.NetworkPlanToSummary(plan)
	var out bytes.Buffer
	NetworkPlanToText(&out, summary, &NetworkPlanToTextOpts{
		Style:                 colors.WithColors,
		Checkmark:             true,
		IncludeSupportServers: showSupportServers,
	})
	return out.String()
}
