// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeblueprint"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

var (
	metricsPort   = flag.Int("metrics-port", 8080, "Port to listen for metrics on.")
	telemetryPort = flag.Int("telemetry-port", 8081, "Port to listen for telemetry on.")
	resources     = []struct {
		ApiGroups []string
		Resources []string
	}{
		{[]string{"certificates.k8s.io"}, []string{"certificatesigningrequests"}},
		{[]string{""}, []string{
			"configmaps", "endpoints", "limitranges", "namespaces", "nodes", "persistentvolumeclaims",
			"persistentvolumes", "pods", "replicationcontrollers", "resourcequotas", "secrets", "services"}},
		{[]string{"batch"}, []string{"cronjobs", "jobs"}},
		{[]string{"extensions", "apps"}, []string{"daemonsets", "deployments", "replicasets"}},
		{[]string{"apps"}, []string{"statefulsets"}},
		{[]string{"autoscaling"}, []string{"horizontalpodautoscalers"}},
		{[]string{"extensions", "networking.k8s.io"}, []string{"ingresses"}},
		{[]string{"admissionregistration.k8s.io"}, []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"}},
		{[]string{"networking.k8s.io"}, []string{"networkpolicies"}},
		{[]string{"policy"}, []string{"poddisruptionbudgets"}},
		{[]string{"storage.k8s.io"}, []string{"storageclasses", "volumeattachments"}},
	}
)

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	provisioning.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	var rscrs uniquestrings.List
	for _, r := range resources {
		for _, rr := range r.Resources {
			rscrs.Add(rr)
		}
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			Args: []string{
				fmt.Sprintf("--port=%d", *metricsPort),
				fmt.Sprintf("--telemetry-port=%d", *telemetryPort),
				fmt.Sprintf("--resources=%s", strings.Join(rscrs.Strings(), ",")),
			},
		},
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			Annotation: []*kubedef.SpecExtension_Annotation{
				{Key: "prometheus.io/scrape", Value: "true"},
				{Key: "prometheus.io/port", Value: fmt.Sprintf("%d", *metricsPort)},
			},
			SecurityContext: &kubedef.SpecExtension_SecurityContext{
				RunAsUser:  65534,
				RunAsGroup: 65534,
				FsGroup:    65534,
			},
		},
	})

	var rules []*rbacv1.PolicyRuleApplyConfiguration
	for _, r := range resources {
		rules = append(rules, rbacv1.PolicyRule().WithAPIGroups(r.ApiGroups...).WithResources(r.Resources...).WithVerbs("list", "watch"))
	}

	if err := (kubeblueprint.GrantKubeACLs{
		DescriptionBase: "kube-state-metrics",
		Rules:           rules,
	}).Compile(req, kubeblueprint.ClusterScope, out); err != nil {
		return err
	}

	return nil
}

func (configuration) Delete(context.Context, provisioning.StackRequest, *provisioning.DeleteOutput) error {
	// XXX unimplemented
	return nil
}
