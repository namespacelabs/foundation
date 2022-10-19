// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/go-ids"
)

type secretCollector struct {
	secretId                 string
	items                    *collector
	requiredGeneratedSecrets []secretRefAndSpec
}

type secretRefAndSpec struct {
	Ref  *schema.PackageRef
	Spec *schema.SecretSpec
}

func newSecretCollector(secretId string) *secretCollector {
	return &secretCollector{secretId: secretId, items: newDataItemCollector()}
}

func (s *secretCollector) allocate(secrets runtime.GroundedSecrets, ref *schema.PackageRef) (string, string, error) {
	contents := secrets.Get(ref)
	if contents == nil {
		return "", "", fnerrors.BadInputError("%q: missing secret value", ref.Canonical())
	}

	if contents.Value != nil {
		key := domainFragLike(ref.PackageName, ref.Name)
		s.items.set(key, contents.Value)

		return s.secretId, key, nil
	}

	if contents.Spec.Generate != nil {
		name, key := s.allocateGenerated(contents.Ref, contents.Spec)
		return name, key, nil
	}

	return "", "", fnerrors.InternalError("don't know how to handle secret %q", ref.Canonical())
}

func (s *secretCollector) allocateGenerated(ref *schema.PackageRef, spec *schema.SecretSpec) (string, string) {
	s.requiredGeneratedSecrets = append(s.requiredGeneratedSecrets, secretRefAndSpec{ref, spec})
	name, key := generatedSecretName(spec.Generate)
	return name, key
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
