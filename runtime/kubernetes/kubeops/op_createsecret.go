// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tools/maketlscert"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/types"
)

func RegisterCreateSecret() {
	ops.RegisterFunc(func(ctx context.Context, env planning.Context, d *schema.SerializedInvocation, create *kubedef.OpCreateSecretConditionally) (*ops.HandleResult, error) {
		if create.Name == "" {
			return nil, fnerrors.InternalError("%s: create.Name is required", d.Description)
		}

		if create.Namespace == "" {
			return nil, fnerrors.InternalError("%s: create.Namespace is required", d.Description)
		}

		cluster, err := kubedef.InjectedKubeCluster(ctx)
		if err != nil {
			return nil, err
		}

		existing, err := fetchResource(ctx, cluster.RESTConfig(), d.Description, inlineClass("secrets"), create.Name, create.Namespace, schema.PackageNames(d.Scope...))
		if err != nil {
			return nil, err
		}

		if existing != nil {
			return nil, nil // Nothing to do.
		}

		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      create.Name,
				Namespace: create.Namespace,
				Labels:    kubedef.MakeLabels(env.Environment(), nil),
			},
		}

		if create.SelfSignedCertificate != nil {
			bundle, err := maketlscert.CreateSelfSignedCertificateChain(ctx, env.Environment(), create.SelfSignedCertificate)
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
			resource, err := ops.Value[*types.Resource](d, "value")
			if err != nil {
				return nil, fnerrors.New("%s: failed to retrieve value: %w", d.Description, err)
			}

			if resource.GetContents() == nil {
				return nil, fnerrors.BadInputError("%s: resource is missing a value", d.Description)
			}

			newSecret.Data = map[string][]byte{
				create.UserSpecifiedName: resource.GetContents(),
			}
		}

		if _, err := cluster.Client().CoreV1().Secrets(create.Namespace).Create(ctx, newSecret, metav1.CreateOptions{
			FieldManager: kubedef.K8sFieldManager,
		}); err != nil {
			return nil, err
		}

		return nil, nil
	})
}

type inlineClass string

func (s inlineClass) GetResource() string                      { return string(s) }
func (s inlineClass) GetResourceClass() *kubedef.ResourceClass { return nil }
