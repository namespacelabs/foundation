// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeops

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tools/maketlscert"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/types"
)

func RegisterCreateSecret() {
	execution.RegisterFuncs(execution.Funcs[*kubedef.OpCreateSecretConditionally]{
		Handle: func(ctx context.Context, d *fnschema.SerializedInvocation, create *kubedef.OpCreateSecretConditionally) (*execution.HandleResult, error) {
			if create.Name == "" {
				return nil, fnerrors.InternalError("%s: create.Name is required", d.Description)
			}

			if create.Namespace == "" {
				return nil, fnerrors.InternalError("%s: create.Namespace is required", d.Description)
			}

			cluster, err := kubedef.InjectedKubeClusterNamespace(ctx)
			if err != nil {
				return nil, err
			}

			kubecluster := cluster.Cluster().(kubedef.KubeCluster)

			existing, err := fetchResource(ctx, kubecluster,
				d.Description, schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
				create.Name, create.Namespace, fnschema.PackageNames(d.Scope...))
			if err != nil {
				return nil, err
			}

			if existing != nil {
				return nil, nil // Nothing to do.
			}

			kubecfg := cluster.KubeConfig()

			newSecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        create.Name,
					Namespace:   create.Namespace,
					Labels:      kubedef.MakeLabels(kubecfg.Environment, nil),
					Annotations: kubedef.BaseAnnotations(),
				},
			}

			if create.SelfSignedCertificate != nil {
				bundle, err := maketlscert.CreateSelfSignedCertificateChain(ctx, kubecfg.Environment, create.SelfSignedCertificate)
				if err != nil {
					return nil, err
				}

				bundleBytes, err := json.Marshal(bundle)
				if err != nil {
					return nil, err
				}

				newSecret.Data = map[string][]byte{
					create.UserSpecifiedName: bundleBytes,
				}
			} else {
				resource, err := execution.ComputedValue[*types.Resource](d, "value")
				if err != nil {
					return nil, fnerrors.Newf("%s: failed to retrieve value: %w", d.Description, err)
				}

				if resource.GetContents() == nil {
					return nil, fnerrors.BadInputError("%s: resource is missing a value", d.Description)
				}

				newSecret.Data = map[string][]byte{
					create.UserSpecifiedName: resource.GetContents(),
				}
			}

			if _, err := kubecluster.PreparedClient().Clientset.CoreV1().Secrets(create.Namespace).Create(ctx, newSecret, metav1.CreateOptions{
				FieldManager: kubedef.K8sFieldManager,
			}); err != nil {
				return nil, err
			}

			return nil, nil
		},

		PlanOrder: func(ctx context.Context, create *kubedef.OpCreateSecretConditionally) (*fnschema.ScheduleOrder, error) {
			// Same as secrets.
			return kubedef.PlanOrder(schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, create.Namespace, create.Name), nil
		},
	})
}
