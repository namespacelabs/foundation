// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubenaming"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/go-ids"
)

type secretCollector struct {
	secrets                  secrets.GroundedSecrets
	secretId                 string
	items                    *collector
	requiredGeneratedSecrets []secretRefAndSpec
}

type secretRefAndSpec struct {
	Ref  *schema.PackageRef
	Spec *schema.SecretSpec
}

func newSecretCollector(secs secrets.GroundedSecrets, secretId string) *secretCollector {
	return &secretCollector{secrets: secs, secretId: secretId, items: newDataItemCollector()}
}

type secretReference struct {
	Name string // Secret object name.
	Key  string // Key within the Secret above that this secret refers to.
}

func (s *secretCollector) allocate(ctx context.Context, ref *schema.PackageRef) (*secretReference, error) {
	contents, err := s.secrets.Get(ctx, ref)
	if err != nil {
		return nil, err
	}

	if contents.Value != nil {
		key := kubenaming.DomainFragLike(ref.PackageName, ref.Name)
		s.items.set(key, contents.Value)

		return &secretReference{Name: s.secretId, Key: key}, nil
	}

	if contents.Spec.Generate != nil {
		alloc := s.allocateGenerated(contents.Ref, contents.Spec)
		return alloc, nil
	}

	return nil, fnerrors.InternalError("don't know how to handle secret %q", ref.Canonical())
}

func (s *secretCollector) allocateGenerated(ref *schema.PackageRef, spec *schema.SecretSpec) *secretReference {
	s.requiredGeneratedSecrets = append(s.requiredGeneratedSecrets, secretRefAndSpec{ref, spec})
	name, key := generatedSecretName(spec.Generate)
	return &secretReference{Name: name, Key: key}
}

func (s *secretCollector) planDeployment(ns string, annotations, labels map[string]string) []definition {
	var operations []definition

	if (len(s.items.data) + len(s.items.binaryData)) > 0 {
		operations = append(operations, kubedef.Apply{
			Description: "Server secrets",
			Resource: applycorev1.Secret(s.secretId, ns).
				WithAnnotations(annotations).
				WithLabels(labels).
				WithLabels(map[string]string{
					kubedef.K8sKind: kubedef.K8sStaticConfigKind,
				}).
				WithStringData(s.items.data).
				WithData(s.items.binaryData),
		})
	}

	for _, grounded := range s.requiredGeneratedSecrets {
		gen := grounded.Spec
		name, key := generatedSecretName(gen.Generate)

		data := map[string][]byte{}
		switch gen.Generate.Format {
		case schema.SecretSpec_GenerateSpec_FORMAT_BASE32:
			data[key] = []byte(ids.NewRandomBase32ID(int(gen.Generate.RandomByteCount)))
		default: // Including BASE64
			raw := make([]byte, gen.Generate.RandomByteCount)
			_, _ = rand.Reader.Read(raw)
			data[key] = []byte(base64.RawStdEncoding.EncodeToString(raw))
		}

		operations = append(operations, kubedef.Create{
			Description:         fmt.Sprintf("Generated secret: %s", grounded.Ref.Canonical()),
			SetNamespace:        true,
			SkipIfAlreadyExists: true,
			Resource:            "secrets",
			Body: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   ns,
					Annotations: annotations,
					Labels:      labels,
				},
				Data: data,
			},
		})
	}

	return operations
}

func generatedSecretName(spec *schema.SecretSpec_GenerateSpec) (string, string) {
	return fmt.Sprintf("gen-%s", spec.UniqueId), "generated"
}
