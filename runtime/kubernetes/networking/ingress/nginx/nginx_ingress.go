// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nginx

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jet/kube-webhook-certgen/pkg/certs"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionregistrationv1 "k8s.io/client-go/applyconfigurations/admissionregistration/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	//go:embed ingress.yaml
	lib embed.FS
)

func RegisterGraphHandlers() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, g *schema.Definition, op *OpGenerateWebhookCert) (*ops.DispatcherResult, error) {
		cfg, err := client.ComputeHostEnv(env.DevHost(), env.Proto())
		if err != nil {
			return nil, err
		}

		cli, err := client.NewClientFromHostEnv(cfg)
		if err != nil {
			return nil, err
		}

		if err := tasks.Action("nginx.apply-namespace").Run(ctx, func(ctx context.Context) error {
			_, err := cli.CoreV1().Namespaces().Apply(ctx, corev1.Namespace(op.Namespace).WithLabels(map[string]string{
				"app.kubernetes.io/name":     "ingress-nginx",
				"app.kubernetes.io/instance": "ingress-nginx",
			}), kubedef.Ego())
			return err
		}); err != nil {
			return nil, fnerrors.RemoteError("nginx: failed to ensure namespace: %w", err)
		}

		if err := tasks.Action("nginx.generate-webhook").HumanReadablef(g.Description).Run(ctx, func(ctx context.Context) error {
			webhook := &admissionregistrationv1.ValidatingWebhookConfigurationApplyConfiguration{}
			if err := json.Unmarshal(op.WebhookDefinition, webhook); err != nil {
				return fnerrors.InternalError("nginx: failed to deserialize webhook definition: %w", err)
			}

			secret, err := cli.CoreV1().Secrets(op.Namespace).Get(ctx, op.SecretName, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				newCa, newCert, newKey := certs.GenerateCerts(op.TargetHost)
				newSecret := &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      op.SecretName,
						Namespace: op.Namespace,
					},
					Data: map[string][]byte{"ca": newCa, "cert": newCert, "key": newKey},
				}

				_, err := cli.CoreV1().Secrets(op.Namespace).Create(ctx, newSecret, metav1.CreateOptions{
					FieldManager: kubedef.Ego().FieldManager,
				})
				if err != nil {
					return fnerrors.RemoteError("nginx: failed to create secret: %w", err)
				}

				secret = newSecret
			} else if err != nil {
				return fnerrors.RemoteError("nginx: failed to get secret: %w", err)
			}

			for _, webhook := range webhook.Webhooks {
				webhook.ClientConfig.WithCABundle(secret.Data["ca"]...)
			}

			if _, err := cli.AdmissionregistrationV1().ValidatingWebhookConfigurations().Apply(ctx, webhook, kubedef.Ego()); err != nil {
				return fnerrors.RemoteError("nginx: failed to apply webhook: %w", err)
			}

			return nil
		}); err != nil {
			return nil, err
		}

		return nil, nil
	})
}

func Ensure(ctx context.Context) ([]*schema.Definition, error) {
	f, err := lib.Open("ingress.yaml")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	applies, err := kubeparser.FromReader("nginx Ingress", f)
	if err != nil {
		return nil, err
	}

	var defs []*schema.Definition
	var service, webhook kubedef.Apply
	for _, apply := range applies {
		if apply.Resource == "validatingwebhookconfigurations" && apply.Name == "ingress-nginx-admission" {
			webhook = apply
			continue
		} else if apply.Resource == "services" && apply.Name == "ingress-nginx-controller-admission" {
			service = apply
		}

		def, err := apply.ToDefinition()
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}

	if webhook.Body == nil {
		return nil, fnerrors.InternalError("nginx: webhook definition is missing")
	}

	if service.Body == nil {
		return nil, fnerrors.InternalError("nginx: service definition is missing")
	}

	serializedWebhook, err := json.Marshal(webhook.Body)
	if err != nil {
		return nil, fnerrors.InternalError("nginx: failed to serialize webhook: %w", err)
	}

	op, err := anypb.New(&OpGenerateWebhookCert{
		Namespace:         service.Namespace,
		SecretName:        "ingress-nginx-admission",
		WebhookDefinition: serializedWebhook,
		TargetHost:        fmt.Sprintf("ingress-nginx-controller-admission,ingress-nginx-controller-admission.%s.svc", service.Namespace),
	})

	if err != nil {
		return nil, fnerrors.InternalError("nginx: failed to serialize OpGenerateWebhookCert: %w", err)
	}

	// It's important that we create the webhook + CAbundle first, so it's available to the nginx deployment.
	return append([]*schema.Definition{{Description: "nginx Ingress: Namespace + Webhook + CABundle", Impl: op}}, defs...), nil
}

func IngressAnnotations(hasTLS bool, backendProtocol string, extensions []*anypb.Any) (map[string]string, error) {
	annotations := map[string]string{
		"kubernetes.io/ingress.class":                  "nginx",
		"nginx.ingress.kubernetes.io/backend-protocol": strings.ToUpper(backendProtocol),
	}

	if hasTLS {
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "true"
	} else {
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "false"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "false"
	}

	var cors *schema.HttpCors

	for _, ext := range extensions {
		switch ext.TypeUrl {
		case "type.googleapis.com/foundation.schema.HttpCors":
			corsConf := &schema.HttpCors{}
			if err := ext.UnmarshalTo(corsConf); err != nil {
				return nil, fnerrors.InternalError("nginx: failed to unpack CORS configuration: %v", err)
			}

			if cors == nil {
				cors = corsConf
			} else if !proto.Equal(cors, corsConf) {
				return nil, fnerrors.InternalError("nginx: incompatible CORS configurations")
			}

		default:
			return nil, fnerrors.InternalError("nginx: don't know how to handle extension %q", ext.TypeUrl)
		}
	}

	if cors != nil {
		// XXX validate allowed origin syntax.

		annotations["nginx.ingress.kubernetes.io/enable-cors"] = "true"
		annotations["nginx.ingress.kubernetes.io/cors-allow-origin"] = strings.Join(cors.AllowedOrigin, ", ")
	}

	return annotations, nil
}

type NameRef struct {
	Namespace, ServiceName string
	ContainerPort          int
}

func IngressLoadBalancerService() *NameRef {
	return &NameRef{
		Namespace:     "ingress-nginx",
		ServiceName:   "ingress-nginx-controller",
		ContainerPort: 80,
	}
}

func ControllerSelector() map[string]string {
	return map[string]string{"app.kubernetes.io/component": "controller"}
}
