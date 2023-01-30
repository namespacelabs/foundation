// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/providers/nscloud/nsingress"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/shared"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/execution/defs"
	"namespacelabs.dev/foundation/std/tasks"
)

var (
	//go:embed ingress.yaml ingress_webhook.yaml
	lib embed.FS
)

func RegisterGraphHandlers() {
	execution.RegisterFuncs(execution.Funcs[*OpGenerateWebhookCert]{
		Handle: func(ctx context.Context, g *schema.SerializedInvocation, op *OpGenerateWebhookCert) (*execution.HandleResult, error) {
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
				return nil, fnerrors.InvocationError("kubernetes", "nginx: failed to ensure namespace: %w", err)
			}

			if err := tasks.Action("nginx.generate-webhook").HumanReadablef(g.Description).Run(ctx, func(ctx context.Context) error {
				webhook := &admissionregistrationv1.ValidatingWebhookConfigurationApplyConfiguration{}
				if err := json.Unmarshal(op.WebhookDefinition, webhook); err != nil {
					return fnerrors.InternalError("nginx: failed to deserialize webhook definition: %w", err)
				}

				cli := cluster.PreparedClient().Clientset
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
						return fnerrors.InvocationError("kubernetes", "nginx: failed to create secret: %w", err)
					}

					secret = newSecret
				} else if err != nil {
					return fnerrors.InvocationError("kubernetes", "nginx: failed to get secret: %w", err)
				}

				for _, webhook := range webhook.Webhooks {
					webhook.ClientConfig.WithCABundle(secret.Data["ca"]...)
				}

				if _, err := cli.AdmissionregistrationV1().ValidatingWebhookConfigurations().Apply(ctx, webhook, kubedef.Ego()); err != nil {
					return fnerrors.InvocationError("kubernetes", "nginx: failed to apply webhook: %w", err)
				}

				return nil
			}); err != nil {
				return nil, err
			}

			return nil, nil
		},

		PlanOrder: func(ctx context.Context, _ *OpGenerateWebhookCert) (*schema.ScheduleOrder, error) {
			return &schema.ScheduleOrder{
				SchedAfterCategory: []string{
					kubedef.MakeSchedCat(kubeschema.GroupKind{Kind: "Namespace"}),
				},
			}, nil
		},
	})
}

type nginx struct {
	shared.MapPublicLoadBalancer
}

func Ingress() kubedef.IngressClass {
	return nginx{}
}

func (nginx) Name() string { return "nginx" }

func (nginx) ComputeNaming(env *schema.Environment, naming *schema.Naming) (*schema.ComputedNaming, error) {
	return nsingress.ComputeNaming(env, naming)
}

func (nginx) Ensure(ctx context.Context) ([]*schema.SerializedInvocation, error) {
	f, err := lib.Open("ingress.yaml")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	applies, err := kubeparser.MultipleFromReader("nginx Ingress", f, true)
	if err != nil {
		return nil, err
	}

	webhookDef, err := fs.ReadFile(lib, "ingress_webhook.yaml")
	if err != nil {
		return nil, err
	}

	webhook, err := kubeparser.Single(webhookDef, true)
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

func (nginx) Annotate(ns, name string, domains []*schema.Domain, hasTLS bool, backendProtocol kubedef.BackendProtocol, extensions []*anypb.Any) (*kubedef.IngressAnnotations, error) {
	return Annotate(hasTLS, backendProtocol, extensions)
}

func Annotate(hasTLS bool, backendProtocol kubedef.BackendProtocol, extensions []*anypb.Any) (*kubedef.IngressAnnotations, error) {
	annotations := kubedef.BaseAnnotations()

	annotations["kubernetes.io/ingress.class"] = "nginx"
	annotations["nginx.ingress.kubernetes.io/backend-protocol"] = strings.ToUpper(string(backendProtocol))

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

	annotations["nginx.ingress.kubernetes.io/proxy-read-timeout"] = "3600"
	annotations["nginx.ingress.kubernetes.io/proxy-send-timeout"] = "3600"

	return &kubedef.IngressAnnotations{Annotations: annotations}, nil
}

func (nginx) Service() *kubedef.IngressSelector {
	return &kubedef.IngressSelector{
		Namespace:     "ingress-nginx",
		ServiceName:   "ingress-nginx-controller",
		ContainerPort: 80,
		PodSelector:   map[string]string{"app.kubernetes.io/component": "controller"},
	}
}

func (n nginx) Waiter(restcfg *rest.Config) kubedef.KubeIngressWaiter {
	return kubeobserver.WaitOnResource{
		RestConfig:       restcfg,
		Description:      "Ingress Controller (nginx)",
		Namespace:        n.Service().Namespace,
		Name:             n.Service().ServiceName,
		GroupVersionKind: kubeschema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		Scope:            "namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx",
	}
}
