// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
)

const legacyCreateManager = "kubectl-create"

// migrateLegacyEnvOwnership lets the following server-side apply prune obsolete environment entries without adopting unrelated legacy fields.
func migrateLegacyEnvOwnership(ctx context.Context, client rest.Interface, resource schema.GroupVersionResource, namespace, name string, desiredJSON []byte) error {
	var desired unstructured.Unstructured
	if err := json.Unmarshal(desiredJSON, &desired); err != nil {
		return fmt.Errorf("failed to parse desired resource: %w", err)
	}

	if !supportsLegacyEnvMigration(&desired) {
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		req := client.Get()
		if namespace != "" {
			req = req.Namespace(namespace)
		}
		req = req.Resource(resource.Resource).Name(name)

		var live unstructured.Unstructured
		if err := req.Do(ctx).Into(&live); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to read existing resource: %w", err)
		}

		patch, err := legacyEnvOwnershipPatch(&live, &desired)
		if err != nil {
			return err
		}
		if patch == nil {
			return nil
		}

		patchReq := client.Patch(types.JSONPatchType)
		if namespace != "" {
			patchReq = patchReq.Namespace(namespace)
		}
		patchReq = patchReq.Resource(resource.Resource).Name(name).Body(patch)
		if err := patchReq.Do(ctx).Error(); err != nil {
			return fmt.Errorf("failed to migrate legacy environment ownership: %w", err)
		}

		return nil
	})
}

func supportsLegacyEnvMigration(obj *unstructured.Unstructured) bool {
	gvk := obj.GroupVersionKind()
	if gvk.GroupVersion().String() != "apps/v1" {
		return false
	}

	switch gvk.Kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return obj.GetLabels()[kubedef.AppKubernetesIoManagedBy] == kubedef.ManagerId && obj.GetLabels()[kubedef.K8sServerId] != ""
	default:
		return false
	}
}

func legacyEnvOwnershipPatch(live, desired *unstructured.Unstructured) ([]byte, error) {
	if !supportsLegacyEnvMigration(live) || !supportsLegacyEnvMigration(desired) ||
		live.GetLabels()[kubedef.K8sServerId] != desired.GetLabels()[kubedef.K8sServerId] {
		return nil, nil
	}

	stalePaths, err := staleEnvPaths(live, desired)
	if err != nil || len(stalePaths) == 0 {
		return nil, err
	}

	managedFields := live.GetManagedFields()
	fieldSets := make([]*fieldpath.Set, len(managedFields))
	for i, entry := range managedFields {
		if entry.FieldsType != "FieldsV1" || entry.FieldsV1 == nil {
			continue
		}

		fieldSets[i] = fieldpath.NewSet()
		if err := fieldSets[i].FromJSON(bytes.NewReader(entry.FieldsV1.Raw)); err != nil {
			return nil, fmt.Errorf("failed to decode managed fields for %q: %w", entry.Manager, err)
		}
	}

	var transfer *fieldpath.Set
	for i, entry := range managedFields {
		if entry.Manager != legacyCreateManager || entry.Operation != metav1.ManagedFieldsOperationUpdate ||
			entry.Subresource != "" || entry.APIVersion != desired.GetAPIVersion() || fieldSets[i] == nil {
			continue
		}

		owned := fieldpath.NewSet()
		for _, path := range stalePaths {
			candidate := fieldsWithPrefix(fieldSets[i], path)
			if candidate.Empty() {
				continue
			}

			shared := false
			for j, other := range managedFields {
				if j == i || fieldSets[j] == nil || other.Manager == kubedef.K8sFieldManager ||
					(other.Manager == legacyCreateManager && other.Operation == metav1.ManagedFieldsOperationUpdate) {
					continue
				}
				if !fieldsWithPrefix(fieldSets[j], path).Empty() {
					shared = true
					break
				}
			}
			if !shared {
				owned = owned.Union(candidate)
			}
		}
		if owned.Empty() {
			continue
		}

		if transfer == nil {
			transfer = owned
		} else {
			transfer = transfer.Union(owned)
		}
		fieldSets[i] = fieldSets[i].Difference(owned)
	}

	if transfer == nil || transfer.Empty() {
		return nil, nil
	}

	foundationIndex := -1
	for i, entry := range managedFields {
		if entry.Manager == kubedef.K8sFieldManager && entry.Operation == metav1.ManagedFieldsOperationApply &&
			entry.Subresource == "" && entry.APIVersion == desired.GetAPIVersion() && fieldSets[i] != nil {
			foundationIndex = i
			break
		}
	}

	if foundationIndex == -1 {
		managedFields = append(managedFields, metav1.ManagedFieldsEntry{
			Manager:    kubedef.K8sFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			APIVersion: desired.GetAPIVersion(),
			FieldsType: "FieldsV1",
		})
		fieldSets = append(fieldSets, transfer)
		foundationIndex = len(managedFields) - 1
	} else {
		fieldSets[foundationIndex] = fieldSets[foundationIndex].Union(transfer)
	}

	updated := make([]metav1.ManagedFieldsEntry, 0, len(managedFields))
	for i, entry := range managedFields {
		if fieldSets[i] != nil {
			raw, err := fieldSets[i].ToJSON()
			if err != nil {
				return nil, fmt.Errorf("failed to encode managed fields for %q: %w", entry.Manager, err)
			}
			if fieldSets[i].Empty() {
				continue
			}
			entry.FieldsV1 = &metav1.FieldsV1{Raw: raw}
		}
		updated = append(updated, entry)
	}

	return json.Marshal([]map[string]any{
		{"op": "replace", "path": "/metadata/managedFields", "value": updated},
		{"op": "replace", "path": "/metadata/resourceVersion", "value": live.GetResourceVersion()},
	})
}

func staleEnvPaths(live, desired *unstructured.Unstructured) ([]fieldpath.Path, error) {
	var paths []fieldpath.Path
	for _, field := range []string{"containers", "initContainers"} {
		liveContainers, err := namedContainerEnvs(live, field)
		if err != nil {
			return nil, err
		}
		desiredContainers, err := namedContainerEnvs(desired, field)
		if err != nil {
			return nil, err
		}

		for containerName, liveEnv := range liveContainers {
			desiredEnv, ok := desiredContainers[containerName]
			if !ok {
				continue
			}
			for envName := range liveEnv {
				if _, ok := desiredEnv[envName]; ok {
					continue
				}

				paths = append(paths, fieldpath.Path{
					fieldpath.FieldNameElement("spec"),
					fieldpath.FieldNameElement("template"),
					fieldpath.FieldNameElement("spec"),
					fieldpath.FieldNameElement(field),
					fieldpath.KeyElementByFields("name", containerName),
					fieldpath.FieldNameElement("env"),
					fieldpath.KeyElementByFields("name", envName),
				})
			}
		}
	}
	return paths, nil
}

func fieldsWithPrefix(fields *fieldpath.Set, prefix fieldpath.Path) *fieldpath.Set {
	matched := fieldpath.NewSet()
	fields.Iterate(func(path fieldpath.Path) {
		if len(path) < len(prefix) {
			return
		}
		for i := range prefix {
			if !path[i].Equals(prefix[i]) {
				return
			}
		}
		matched.Insert(path)
	})
	return matched
}

func namedContainerEnvs(obj *unstructured.Unstructured, field string) (map[string]map[string]struct{}, error) {
	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", field)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", field, err)
	}
	if !found {
		return map[string]map[string]struct{}{}, nil
	}

	result := make(map[string]map[string]struct{}, len(containers))
	for _, rawContainer := range containers {
		container, ok := rawContainer.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid %s entry", field)
		}
		name, _, _ := unstructured.NestedString(container, "name")
		if name == "" {
			continue
		}

		envNames := map[string]struct{}{}
		env, _, err := unstructured.NestedSlice(container, "env")
		if err != nil {
			return nil, fmt.Errorf("failed to read %s %q environment: %w", field, name, err)
		}
		for _, rawEnv := range env {
			envEntry, ok := rawEnv.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid environment entry in %s %q", field, name)
			}
			envName, _, _ := unstructured.NestedString(envEntry, "name")
			if envName != "" {
				envNames[envName] = struct{}{}
			}
		}
		result[name] = envNames
	}
	return result, nil
}
