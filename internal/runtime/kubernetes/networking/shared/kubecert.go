// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package shared

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution/defs"
)

func MakeCertificateSecrets(ns string, domain *schema.Domain, cert *schema.Certificate) map[string]kubedef.IngressCertificate {
	certSecrets := map[string]kubedef.IngressCertificate{} // Map fqdn->secret name.

	name := fmt.Sprintf("tls-%s", strings.ReplaceAll(domain.Fqdn, ".", "-"))
	certSecrets[domain.Fqdn] = kubedef.IngressCertificate{
		SecretName: name,
		Defs: []defs.MakeDefinition{
			kubedef.Apply{
				Description: fmt.Sprintf("Certificate for %s", domain.Fqdn),
				Resource: applycorev1.
					Secret(name, ns).
					WithType(corev1.SecretTypeTLS).
					WithLabels(kubedef.ManagedByUs()).
					WithAnnotations(kubedef.BaseAnnotations()).
					WithData(map[string][]byte{
						"tls.key": cert.PrivateKey,
						"tls.crt": cert.CertificateBundle,
					}),
			},
		},
	}

	return certSecrets
}
