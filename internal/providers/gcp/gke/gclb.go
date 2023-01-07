// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gke

import (
	"context"

	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/providers/nscloud/nsingress"
	"namespacelabs.dev/foundation/schema"
)

type gclb struct{}

func (gclb) ComputeNaming(env *schema.Environment, naming *schema.Naming) (*schema.ComputedNaming, error) {
	return nsingress.ComputeNaming(env, naming)
}

func (gclb) Ensure(context.Context) ([]*schema.SerializedInvocation, error) {
	return nil, nil
}
func (gclb) Service() *kubedef.IngressSelector             { return nil }
func (gclb) Waiter(*rest.Config) kubedef.KubeIngressWaiter { return nil }
func (gclb) Map(ctx context.Context, domain *schema.Domain, ns, name string) ([]*kubedef.OpMapAddress, error) {
	return nil, nil
}
