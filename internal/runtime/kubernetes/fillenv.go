// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	rtschema "namespacelabs.dev/foundation/schema/runtime"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

type fillContainer struct {
	container *applycorev1.ContainerApplyConfiguration
	ensure    *kubedef.EnsureDeployment
}

func (x *fillContainer) SetValue(key, value string) error {
	entry := applycorev1.EnvVar().WithName(key).WithValue(value)
	x.container = x.container.WithEnv(entry)
	return nil
}

func (x *fillContainer) SetSecret(key string, secret *runtime.SecretRef) error {
	entry := applycorev1.EnvVar().WithName(key).
		WithValueFrom(applycorev1.EnvVarSource().WithSecretKeyRef(
			applycorev1.SecretKeySelector().WithName(secret.Name).WithKey(secret.Key)))
	x.container = x.container.WithEnv(entry)
	return nil
}

func (x *fillContainer) SetExperimentalFromDownwardsFieldPath(key, value string) error {
	entry := applycorev1.EnvVar().WithName(key).
		WithValueFrom(applycorev1.EnvVarSource().WithFieldRef(
			applycorev1.ObjectFieldSelector().WithFieldPath(value)))
	x.container = x.container.WithEnv(entry)
	return nil
}

func (x *fillContainer) SetLateBoundResourceFieldSelector(key string, sel runtimepb.SetContainerField_ValueSource, src *schema.ResourceConfigFieldSelector) error {
	x.ensure.SetContainerFields = append(x.ensure.SetContainerFields, &rtschema.SetContainerField{
		SetEnv: []*rtschema.SetContainerField_SetValue{
			{
				ContainerName:               *x.container.Name,
				Key:                         key,
				Value:                       sel,
				ResourceConfigFieldSelector: src,
			},
		},
	})
	return nil
}

func fillEnv(ctx context.Context, rt *runtimepb.RuntimeConfig, container *applycorev1.ContainerApplyConfiguration, env []*schema.BinaryConfig_EnvEntry, secrets *secretCollector, ensure *kubedef.EnsureDeployment) (*applycorev1.ContainerApplyConfiguration, error) {
	x := fillContainer{container, ensure}

	if err := runtime.ResolveResolvables(ctx, rt, secrets, env, &x); err != nil {
		return nil, err
	}

	return x.container, nil
}
