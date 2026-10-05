// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeops

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
)

func TestLegacyEnvOwnershipPatch(t *testing.T) {
	containerPath := containerFieldPath("containers", "server")
	obsoletePath := envFieldPath("containers", "server", "OBSOLETE")
	retainedPath := envFieldPath("containers", "server", "RETAINED")
	live := workloadWithEnv("11", "OBSOLETE", "RETAINED")
	live.SetManagedFields([]metav1.ManagedFieldsEntry{
		managedFieldsEntry(legacyCreateManager, metav1.ManagedFieldsOperationUpdate, fieldpath.NewSet(
			containerPath,
			obsoletePath,
			appendPath(obsoletePath, "name"),
			appendPath(obsoletePath, "valueFrom", "secretKeyRef", "key"),
			appendPath(obsoletePath, "valueFrom", "secretKeyRef", "name"),
			retainedPath,
			appendPath(retainedPath, "name"),
			appendPath(retainedPath, "value"),
		)),
		managedFieldsEntry(kubedef.K8sFieldManager, metav1.ManagedFieldsOperationApply, fieldpath.NewSet(
			containerPath,
			retainedPath,
			appendPath(retainedPath, "name"),
			appendPath(retainedPath, "value"),
		)),
		managedFieldsEntry("image-controller", metav1.ManagedFieldsOperationApply, fieldpath.NewSet(
			containerPath,
			appendPath(containerPath, "image"),
		)),
	})
	desired := workloadWithEnv("", "RETAINED")
	assert.True(t, supportsLegacyEnvMigration(live))
	assert.True(t, supportsLegacyEnvMigration(desired))
	paths, err := staleEnvPaths(live, desired)
	assert.NoError(t, err)
	assert.Len(t, paths, 1)

	patch, err := legacyEnvOwnershipPatch(live, desired)
	if !assert.NoError(t, err) || !assert.NotEmpty(t, patch) {
		return
	}

	entries := managedFieldsFromPatch(t, patch)
	legacy := managedFieldSet(t, entries, legacyCreateManager)
	foundation := managedFieldSet(t, entries, kubedef.K8sFieldManager)
	assert.False(t, legacy.Has(obsoletePath))
	assert.True(t, legacy.Has(containerPath))
	assert.True(t, legacy.Has(retainedPath))
	assert.True(t, foundation.Has(obsoletePath))
	assert.True(t, foundation.Has(appendPath(obsoletePath, "valueFrom", "secretKeyRef", "key")))
	assert.True(t, foundation.Has(retainedPath))

	live.SetManagedFields(entries)
	secondPatch, err := legacyEnvOwnershipPatch(live, desired)
	assert.NoError(t, err)
	assert.Empty(t, secondPatch)
}

func TestLegacyEnvOwnershipPatchCreatesFoundationManager(t *testing.T) {
	obsoletePath := envFieldPath("initContainers", "setup", "OBSOLETE")
	live := workloadWithInitEnv("29", "OBSOLETE")
	live.SetManagedFields([]metav1.ManagedFieldsEntry{
		managedFieldsEntry(legacyCreateManager, metav1.ManagedFieldsOperationUpdate, fieldpath.NewSet(
			obsoletePath,
			appendPath(obsoletePath, "name"),
			appendPath(obsoletePath, "value"),
		)),
	})
	desired := workloadWithInitEnv("")

	patch, err := legacyEnvOwnershipPatch(live, desired)
	if !assert.NoError(t, err) || !assert.NotEmpty(t, patch) {
		return
	}

	entries := managedFieldsFromPatch(t, patch)
	assert.Len(t, entries, 1)
	assert.Equal(t, kubedef.K8sFieldManager, entries[0].Manager)
	assert.Equal(t, metav1.ManagedFieldsOperationApply, entries[0].Operation)
	assert.True(t, managedFieldSet(t, entries, kubedef.K8sFieldManager).Has(obsoletePath))
}

func TestLegacyEnvOwnershipPatchPreservesOtherOwners(t *testing.T) {
	obsoletePath := envFieldPath("containers", "server", "OBSOLETE")
	live := workloadWithEnv("17", "OBSOLETE")
	live.SetManagedFields([]metav1.ManagedFieldsEntry{
		managedFieldsEntry(legacyCreateManager, metav1.ManagedFieldsOperationUpdate, fieldpath.NewSet(
			obsoletePath,
			appendPath(obsoletePath, "name"),
			appendPath(obsoletePath, "valueFrom", "secretKeyRef", "name"),
		)),
		managedFieldsEntry("external-controller", metav1.ManagedFieldsOperationApply, fieldpath.NewSet(
			appendPath(obsoletePath, "valueFrom", "secretKeyRef", "key"),
		)),
	})
	desired := workloadWithEnv("")

	patch, err := legacyEnvOwnershipPatch(live, desired)
	assert.NoError(t, err)
	assert.Empty(t, patch)
}

func workloadWithEnv(resourceVersion string, envNames ...string) *unstructured.Unstructured {
	return workload(resourceVersion, "containers", "server", envNames...)
}

func workloadWithInitEnv(resourceVersion string, envNames ...string) *unstructured.Unstructured {
	return workload(resourceVersion, "initContainers", "setup", envNames...)
}

func workload(resourceVersion, field, containerName string, envNames ...string) *unstructured.Unstructured {
	env := make([]any, 0, len(envNames))
	for _, name := range envNames {
		env = append(env, map[string]any{"name": name, "value": "value"})
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "StatefulSet",
		"metadata": map[string]any{
			"name":            "server-id",
			"resourceVersion": resourceVersion,
			"labels": map[string]any{
				kubedef.AppKubernetesIoManagedBy: kubedef.ManagerId,
				kubedef.K8sServerId:              "id",
			},
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					field: []any{map[string]any{"name": containerName, "env": env}},
				},
			},
		},
	}}
	return obj
}

func envFieldPath(containerField, containerName, envName string) fieldpath.Path {
	return append(containerFieldPath(containerField, containerName),
		fieldpath.FieldNameElement("env"),
		fieldpath.KeyElementByFields("name", envName),
	)
}

func containerFieldPath(containerField, containerName string) fieldpath.Path {
	return fieldpath.Path{
		fieldpath.FieldNameElement("spec"),
		fieldpath.FieldNameElement("template"),
		fieldpath.FieldNameElement("spec"),
		fieldpath.FieldNameElement(containerField),
		fieldpath.KeyElementByFields("name", containerName),
	}
}

func appendPath(path fieldpath.Path, fields ...string) fieldpath.Path {
	result := append(fieldpath.Path(nil), path...)
	for _, field := range fields {
		result = append(result, fieldpath.FieldNameElement(field))
	}
	return result
}

func managedFieldsEntry(manager string, operation metav1.ManagedFieldsOperationType, fields *fieldpath.Set) metav1.ManagedFieldsEntry {
	raw, err := fields.ToJSON()
	if err != nil {
		panic(err)
	}
	return metav1.ManagedFieldsEntry{
		Manager:    manager,
		Operation:  operation,
		APIVersion: "apps/v1",
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: raw},
	}
}

func managedFieldsFromPatch(t *testing.T, patch []byte) []metav1.ManagedFieldsEntry {
	var operations []struct {
		Value json.RawMessage `json:"value"`
	}
	if !assert.NoError(t, json.Unmarshal(patch, &operations)) || !assert.Len(t, operations, 2) {
		return nil
	}

	var entries []metav1.ManagedFieldsEntry
	assert.NoError(t, json.Unmarshal(operations[0].Value, &entries))
	return entries
}

func managedFieldSet(t *testing.T, entries []metav1.ManagedFieldsEntry, manager string) *fieldpath.Set {
	for _, entry := range entries {
		if entry.Manager != manager {
			continue
		}
		set := fieldpath.NewSet()
		assert.NoError(t, set.FromJSON(bytes.NewReader(entry.FieldsV1.Raw)))
		return set
	}
	assert.Fail(t, "managed fields entry not found", manager)
	return fieldpath.NewSet()
}
