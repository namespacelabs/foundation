// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nginx

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/jet/kube-webhook-certgen/pkg/certs"
	"google.golang.org/protobuf/types/known/anypb"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeschema "k8s.io/apimachinery/pkg/runtime/schema"
	admissionregistrationv1 "k8s.io/client-go/applyconfigurations/admissionregistration/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	//go:embed ingress.yaml ingress_webhook.yaml
	lib embed.FS
)

func RegisterGraphHandlers() {
	ops.RegisterFuncs(ops.Funcs[*OpGenerateWebhookCert]{
		Handle: func(ctx context.Context, g *schema.SerializedInvocation, op *OpGenerateWebhookCert) (*ops.HandleResult, error) {
			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			if err := tasks.Action("nginx.apply-namespace").Run(ctx, func(ctx context.Context) error {
				_, err := cluster.PreparedClient().Clientset.CoreV1().Namespaces().Apply(ctx, corev1.Namespace(op.Namespace).WithLabels(map[string]string{
					"app.kubernetes.io/name":     "ingress-nginx",
					"app.kubernetes.io/instance": "ingress-nginx",
				}), kubedef.Ego())
				return err
			}); err != nil {
				return nil, fnerrors.InvocationError("nginx: failed to ensure namespace: %w", err)
			}

			if err := tasks.Action("nginx.generate-webhook").HumanReadablef(g.Description).Run(ctx, func(ctx context.Context) error {
				webhook := &admissionregistrationv1.ValidatingWebhookConfigurationApplyConfiguration{}
				if err := json.Unmarshal(op.WebhookDefinition, webhook); err != nil {
					return fnerrors.InternalError("nginx: failed to deserialize webhook definition: %w", err)
				}

				secret, err := cluster.PreparedClient().Clientset.CoreV1().Secrets(op.Namespace).Get(ctx, op.SecretName, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					newCa, newCert, newKey := certs.GenerateCerts(op.TargetHost)
					newSecret := &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      op.SecretName,
							Namespace: op.Namespace,
						},
						Data: map[string][]byte{"ca": newCa, "cert": newCert, "key": newKey},
					}

					_, err := cluster.PreparedClient().Clientset.CoreV1().Secrets(op.Namespace).Create(ctx, newSecret, metav1.CreateOptions{
						FieldManager: kubedef.Ego().FieldManager,
					})
					if err != nil {
						return fnerrors.InvocationError("nginx: failed to create secret: %w", err)
					}

					secret = newSecret
				} else if err != nil {
					return fnerrors.InvocationError("nginx: failed to get secret: %w", err)
				}

				for _, webhook := range webhook.Webhooks {
					webhook.ClientConfig.WithCABundle(secret.Data["ca"]...)
				}

				if _, err := cluster.PreparedClient().Clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Apply(ctx, webhook, kubedef.Ego()); err != nil {
					return fnerrors.InvocationError("nginx: failed to apply webhook: %w", err)
				}

				return nil
			}); err != nil {
				return nil, err
			}

			return nil, nil
		},

		PlanOrder: func(_ *OpGenerateWebhookCert) (*schema.ScheduleOrder, error) {
			return &schema.ScheduleOrder{
				SchedAfterCategory: []string{
					kubedef.MakeSchedCat(kubeschema.GroupKind{Kind: "Namespace"}),
				},
			}, nil
		},
	})
}

func Ensure(ctx context.Context) ([]*schema.SerializedInvocation, error) {
	f, err := lib.Open("ingress.yaml")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	applies, err := kubeparser.MultipleFromReader("nginx Ingress", f)
	if err != nil {
		return nil, err
	}

	webhookDef, err := fs.ReadFile(lib, "ingress_webhook.yaml")
	if err != nil {
		return nil, err
	}

	webhook, err := kubeparser.Single(webhookDef)
	if err != nil {
		return nil, err
	}

	defs, err := defs.Make(applies...)
	if err != nil {
		return nil, err
	}

	serializedWebhook, err := json.Marshal(webhook.Resource)
	if err != nil {
		return nil, fnerrors.InternalError("nginx: failed to serialize webhook: %w", err)
	}

	const ns = "ingress-nginx"
	op, err := anypb.New(&OpGenerateWebhookCert{
		Namespace:         ns,
		SecretName:        "ingress-nginx-admission",
		WebhookDefinition: serializedWebhook,
		TargetHost:        fmt.Sprintf("ingress-nginx-controller-admission,ingress-nginx-controller-admission.%s.svc", ns),
	})

	if err != nil {
		return nil, fnerrors.InternalError("nginx: failed to serialize OpGenerateWebhookCert: %w", err)
	}

	// It's important that we create the webhook + CAbundle first, so it's available to the nginx deployment.
	return append([]*schema.SerializedInvocation{{Description: "nginx Ingress: Namespace + Webhook + CABundle", Impl: op}}, defs...), nil
}

func IngressAnnotations(hasTLS bool, backendProtocol string, extensions []*anypb.Any) (map[string]string, error) {
	annotations := kubedef.BaseAnnotations()

	annotations["kubernetes.io/ingress.class"] = "nginx"
	annotations["nginx.ingress.kubernetes.io/backend-protocol"] = strings.ToUpper(backendProtocol)

	if hasTLS {
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "true"
	} else {
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "false"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "false"
	}

	var cors *schema.HttpCors
	var entityLimit *ProxyBodySize

	for _, ext := range extensions {
		msg, err := ext.UnmarshalNew()
		if err != nil {
			return nil, fnerrors.InternalError("nginx: failed to unpack configuration: %v", err)
		}

		switch x := msg.(type) {
		case *schema.HttpCors:
			if !protos.CheckConsolidate(x, &cors) {
				return nil, fnerrors.InternalError("nginx: incompatible CORS configurations")
			}

		case *ProxyBodySize:
			if !protos.CheckConsolidate(x, &entityLimit) {
				return nil, fnerrors.InternalError("nginx: incompatible ProxyBodySize configurations")
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

	if entityLimit != nil {
		annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = entityLimit.Limit
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

func IngressWaiter(cfg *rest.Config) kubeobserver.WaitOnResource {
	return kubeobserver.WaitOnResource{
		RestConfig:       cfg,
		Description:      "NGINX Ingress Controller",
		Namespace:        IngressLoadBalancerService().Namespace,
		Name:             IngressLoadBalancerService().ServiceName,
		GroupVersionKind: kubeschema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		Scope:            "namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx",
	}
}
