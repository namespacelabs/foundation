// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"encoding/json"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

type kubernetesRuntimeConfig struct {
	configMap *applycorev1.ConfigMapApplyConfiguration
	volume    *applycorev1.VolumeApplyConfiguration
	mount     *applycorev1.VolumeMountApplyConfiguration
}

func prepareRuntimeConfig(configStruct interface{}, namespace string, resourcePrefix string) (kubernetesRuntimeConfig, error) {
	serializedConfig, err := json.Marshal(configStruct)
	if err != nil {
		return kubernetesRuntimeConfig{}, fnerrors.InternalError("failed to serialize runtime configuration: %w", err)
	}

	configDigest, err := schema.DigestOf(map[string]any{
		"version": runtimeConfigVersion,
		"config":  serializedConfig,
	})
	if err != nil {
		return kubernetesRuntimeConfig{}, fnerrors.InternalError("failed to digest runtime configuration: %w", err)
	}

	configId := resourcePrefix + "-rtconfig-" + configDigest.Hex[:8]

	out := kubernetesRuntimeConfig{}

	out.configMap = applycorev1.ConfigMap(configId, namespace).
		WithLabels(map[string]string{
			kubedef.K8sKind: kubedef.K8sRuntimeConfigKind,
		}).
		WithImmutable(true).
		WithData(map[string]string{
			"runtime.json": string(serializedConfig),
		})

	out.volume = applycorev1.Volume().
		WithName(configId).
		WithConfigMap(applycorev1.ConfigMapVolumeSource().WithName(configId))

	out.mount = applycorev1.VolumeMount().WithMountPath("/namespace/config").WithName(configId).WithReadOnly(true)

	return out, nil
}
