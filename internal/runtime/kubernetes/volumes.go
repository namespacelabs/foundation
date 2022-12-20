// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"fmt"
	"math"

	"github.com/dustin/go-humanize"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

// Transient data structure used to prepare volumes and mounts
type volumeDef struct {
	name string
	// True if the volume is actually a filesync and needs a different handling.
	isWorkspaceSync bool
}

func makePersistentVolume(ns string, env *schema.Environment, loc fnerrors.Location, owner, name, persistentId string, sizeBytes uint64, annotations map[string]string) (*applycorev1.VolumeApplyConfiguration, definitions, error) {
	if sizeBytes >= math.MaxInt64 {
		return nil, nil, fnerrors.NewWithLocation(loc, "requiredstorage value too high (maximum is %d)", math.MaxInt64)
	}

	quantity := resource.NewScaledQuantity(int64(sizeBytes), 0)

	// Ephemeral environments are short lived, so there is no need for persistent volume claims.
	// Admin servers are excluded here as they run as singletons in a global namespace.
	var operations definitions
	var v *applycorev1.VolumeApplyConfiguration

	if env.GetEphemeral() {
		v = applycorev1.Volume().
			WithName(name).
			WithEmptyDir(applycorev1.EmptyDirVolumeSource().
				WithSizeLimit(*quantity))
	} else {
		v = applycorev1.Volume().
			WithName(name).
			WithPersistentVolumeClaim(
				applycorev1.PersistentVolumeClaimVolumeSource().
					WithClaimName(persistentId))

		operations = append(operations, kubedef.Apply{
			Description: fmt.Sprintf("Persistent storage for %s (%s)", owner, humanize.Bytes(sizeBytes)),
			Resource: applycorev1.PersistentVolumeClaim(persistentId, ns).
				WithLabels(kubedef.ManagedByUs()).
				WithAnnotations(annotations).
				WithSpec(applycorev1.PersistentVolumeClaimSpec().
					WithAccessModes(corev1.ReadWriteOnce).
					WithResources(applycorev1.ResourceRequirements().WithRequests(corev1.ResourceList{
						corev1.ResourceStorage: *quantity,
					}))),
		})
	}

	return v, operations, nil
}

func toK8sVol(vol *kubedef.SpecExtension_Volume) (*applycorev1.VolumeApplyConfiguration, error) {
	v := applycorev1.Volume().WithName(vol.Name)
	switch x := vol.VolumeType.(type) {
	case *kubedef.SpecExtension_Volume_Secret_:
		return v.WithSecret(applycorev1.SecretVolumeSource().WithSecretName(x.Secret.SecretName)), nil
	case *kubedef.SpecExtension_Volume_ConfigMap_:
		vol := applycorev1.ConfigMapVolumeSource().WithName(x.ConfigMap.Name)
		for _, it := range x.ConfigMap.Item {
			vol = vol.WithItems(applycorev1.KeyToPath().WithKey(it.Key).WithPath(it.Path))
		}
		return v.WithConfigMap(vol), nil
	default:
		return nil, fnerrors.InternalError("don't know how to instantiate a k8s volume from %v", vol)
	}
}
