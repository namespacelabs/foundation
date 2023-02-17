// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/prototext"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/std/cfg"
)

func newKubernetesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "kubernetes",
	}

	envBound := "dev"
	systemInfo := &cobra.Command{
		Use:  "system-info",
		Args: cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := cfg.LoadContext(root, envBound)
			if err != nil {
				return err
			}

			k, err := kubernetes.ConnectToCluster(ctx, env.Configuration())
			if err != nil {
				return err
			}

			sysInfo, err := k.SystemInfo(ctx)
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), prototext.Format(sysInfo))
			return nil
		}),
	}

	systemInfo.Flags().StringVar(&envBound, "env", envBound, "If specified, produce a env-bound sealed schema.")

	cmd.AddCommand(systemInfo)
	cmd.AddCommand(newObservePodsCmd())

	return cmd
}

func newObservePodsCmd() *cobra.Command {
	var (
		envBound = "dev"
		group    = ""
		version  = "v1"
		resource = "pods"
	)

	cmd := &cobra.Command{
		Use:  "observe",
		Args: cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := cfg.LoadContext(root, envBound)
			if err != nil {
				return err
			}

			k, err := kubernetes.ConnectToCluster(ctx, env.Configuration())
			if err != nil {
				return err
			}

			return listAndWatch(ctx, k.RESTConfig(), schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resource,
			})
		}),
	}

	cmd.Flags().StringVar(&envBound, "env", envBound, "If specified, produce a env-bound sealed schema.")
	cmd.Flags().StringVar(&group, "group", group, "G out of GVR")
	cmd.Flags().StringVar(&version, "version", version, "V out of GVR")
	cmd.Flags().StringVar(&resource, "resource", resource, "R out of GVR")

	return cmd
}

type X interface {
	GetNamespace() string
	GetName() string
	GetResourceVersion() string
}

func listAndWatch(ctx context.Context, cfg *rest.Config, resource schema.GroupVersionResource) error {
	_, client, err := MakeGroupVersionBasedClientAndConfig(ctx, cfg, resource.GroupVersion())
	if err != nil {
		return err
	}

	var opts metav1.ListOptions
	opts.Limit = 20

	onEvent := func(event watch.EventType, pod X) {
		b, _ := json.Marshal(pod)

		fmt.Fprintf(console.Stdout(ctx), "%s [%s/%s] %s [%d bytes]\n", event, pod.GetNamespace(), pod.GetName(), pod.GetResourceVersion(), len(b))
	}

	var wopts metav1.ListOptions
	wopts.Watch = true

	for {
		var result unstructured.UnstructuredList
		if err := client.Get().Resource(resource.Resource).VersionedParams(&opts, scheme.ParameterCodec).Do(ctx).Into(&result); err != nil {
			return err
		}

		for _, x := range result.Items {
			onEvent(watch.Added, &x)
		}

		wopts.ResourceVersion = result.GetResourceVersion()
		if result.GetContinue() == "" {
			break
		} else {
			opts.Continue = result.GetContinue()
		}
	}

	fmt.Fprintf(console.Stdout(ctx), "starting watch\n")

	wintf, err := client.Get().
		Resource(resource.Resource).
		VersionedParams(&wopts, scheme.ParameterCodec).
		Watch(ctx)
	if err != nil {
		return err
	}

	defer wintf.Stop()

	ch := wintf.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case x := <-ch:
			fmt.Fprintf(console.Debug(ctx), "event: %v\n", x)

			switch x.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				if y, ok := x.Object.(X); ok {
					onEvent(x.Type, y)
				} else {
					fmt.Fprintf(console.Stdout(ctx), "failed to handle: %s: %v\n", x.Type, x.Object)
				}

			case watch.Bookmark:

			case watch.Error:
			}
		}
	}
}

func MakeGroupVersionBasedClientAndConfig(ctx context.Context, original *rest.Config, gv schema.GroupVersion) (*rest.Config, rest.Interface, error) {
	config := copyAndSetDefaults(*original, gv)
	client, err := rest.RESTClientFor(config)
	return config, client, err
}

func copyAndSetDefaults(config rest.Config, gv schema.GroupVersion) *rest.Config {
	config.GroupVersion = &gv
	if gv.Group == "" {
		config.APIPath = "/api"
	} else {
		config.APIPath = "/apis"
	}

	r := runtime.NewScheme()
	if err := v1.AddToScheme(r); err != nil {
		panic(err)
	}
	if err := appsv1.AddToScheme(r); err != nil {
		panic(err)
	}

	// config.NegotiatedSerializer = unstructuredscheme.NewUnstructuredNegotiatedSerializer()
	// config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.NegotiatedSerializer = serializer.NewCodecFactory(r).WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return &config
}
