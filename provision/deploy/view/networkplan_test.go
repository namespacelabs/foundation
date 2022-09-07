// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package view

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/provision/deploy/render"
)

func TestNetworkPlanToText(t *testing.T) {
	assertNetworkPlan(t, "empty-deployment", &NetworkPlanToTextOpts{Style: colors.NoColors},
		&render.NetworkPlanSummary{})
	assertNetworkPlan(t, "empty-development", &NetworkPlanToTextOpts{Style: colors.NoColors},
		&render.NetworkPlanSummary{LocalHostname: "mylocalhost"})

	basicSummary := &render.NetworkPlanSummary{
		SupportServices: []*render.NetworkPlanSummary_Service{
			{
				Label:     &render.Label{Label: "Label"},
				LocalPort: 8080,
				AccessCmd: []*render.NetworkPlanSummary_Service_AccessCmd{
					{Cmd: "http://web.example.com/path1"},
					{Cmd: "http://web.example.com/path2"},
				},
			},
			{
				Label: &render.Label{Label: "Longer Label"},
				AccessCmd: []*render.NetworkPlanSummary_Service_AccessCmd{
					{Cmd: "grpcurl hello world long command"},
				},
			},
			{
				Label:       &render.Label{ServiceProto: "short.proto.name"},
				PackageName: "my/package/name",
				AccessCmd:   []*render.NetworkPlanSummary_Service_AccessCmd{{Cmd: "http://localhost"}},
			},
			{
				Label:     &render.Label{ServiceProto: "longlonglonglonglonglonglonglonglonglonglonglonglonglong.proto.name"},
				AccessCmd: []*render.NetworkPlanSummary_Service_AccessCmd{{Cmd: "http://localhost"}},
			},
		},
		FocusedServices: []*render.NetworkPlanSummary_Service{
			{
				Label:     &render.Label{Label: "Focused Label"},
				LocalPort: 8080,
				AccessCmd: []*render.NetworkPlanSummary_Service_AccessCmd{
					{Cmd: "http://web.example.com/path1", IsManaged: true},
					{Cmd: "http://web.example.com/path2", IsManaged: true},
				},
			},
			{
				Label: &render.Label{Label: "Focused Longer Label"},
				AccessCmd: []*render.NetworkPlanSummary_Service_AccessCmd{
					{Cmd: "grpcurl hello world long command", IsManaged: true},
				},
			},
			{
				Label:       &render.Label{ServiceProto: "focused.short.proto.name"},
				PackageName: "my/package/name",
				AccessCmd:   []*render.NetworkPlanSummary_Service_AccessCmd{{Cmd: "http://localhost", IsManaged: true}},
			},
			{
				Label:     &render.Label{ServiceProto: "focused.longlonglonglonglonglonglonglonglonglonglonglonglonglong.proto.name"},
				AccessCmd: []*render.NetworkPlanSummary_Service_AccessCmd{{Cmd: "http://localhost", IsManaged: true}},
			},
		},
	}
	assertNetworkPlan(t, "basic-with-support",
		&NetworkPlanToTextOpts{Style: colors.NoColors, IncludeSupportServers: true, Checkmark: true}, basicSummary)
	assertNetworkPlan(t, "basic-with-support-no-checkmark",
		&NetworkPlanToTextOpts{Style: colors.NoColors, IncludeSupportServers: true, Checkmark: false}, basicSummary)
	assertNetworkPlan(t, "basic-no-support",
		&NetworkPlanToTextOpts{Style: colors.NoColors, IncludeSupportServers: false, Checkmark: true}, basicSummary)

	assertNetworkPlan(t, "with-not-managed-domains",
		&NetworkPlanToTextOpts{Style: colors.NoColors, IncludeSupportServers: false, Checkmark: false},
		&render.NetworkPlanSummary{
			FocusedServices: []*render.NetworkPlanSummary_Service{
				{
					Label: &render.Label{Label: "Focused Label"},
					AccessCmd: []*render.NetworkPlanSummary_Service_AccessCmd{
						{Cmd: "http://web.example.com/path1", IsManaged: true},
						{Cmd: "http://web.example.com/path2", IsManaged: false},
					},
				},
			},
		})
}

func assertNetworkPlan(t *testing.T, snapshotName string, opts *NetworkPlanToTextOpts, r *render.NetworkPlanSummary) {
	var out bytes.Buffer
	NetworkPlanToText(&out, r, opts)

	// Trimming trailing spaces so the `pre-commit's` `trailing-whitespace` check is happy.
	lines := strings.Split(out.String(), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}

	if err := cupaloy.SnapshotMulti(fmt.Sprintf("%s.txt", snapshotName), strings.Join(lines, "\n")); err != nil {
		// Don't use asserts, to avoid triggering t.Fatal, so we can update all screenshots in a single run.
		t.Error(err)
	}
}
