package main

import (
	"context"
	"fmt"
	"path/filepath"

	"google.golang.org/protobuf/types/known/anypb"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/library/kubernetes/rbac"
	"namespacelabs.dev/foundation/schema"
)

type tool struct{}

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleApply(func(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
		intent := &rbac.ClusterRoleIntent{}
		if err := req.UnpackInput(intent); err != nil {
			return err
		}

		source := &protocol.ResourceInstance{}
		if err := req.UnpackInput(source); err != nil {
			return err
		}

		roleName := "ns:user:" + naming.DomainFragLikeN("-", filepath.Base(source.ResourceInstance.PackageName), source.ResourceInstance.Name, naming.StableIDN(source.ResourceInstanceId, 8))
		labels := map[string]string{}

		clusterRole := rbacv1.ClusterRole(roleName).
			WithLabels(labels).
			WithAnnotations(kubedef.BaseAnnotations())

		for _, rule := range intent.Rules {
			r := rbacv1.PolicyRule().WithAPIGroups(rule.ApiGroups...).WithResources(rule.Resources...).WithVerbs(rule.Verbs...)
			clusterRole = clusterRole.WithRules(r)
		}

		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Cluster Role", intent.Name),
			Resource:    clusterRole,
		})

		instance, err := anypb.New(&rbac.ClusterRoleInstance{
			Name: roleName,
		})
		if err != nil {
			return err
		}

		out.OutputResourceInstance = instance
		return nil
	})
	provisioning.Handle(h)
}
