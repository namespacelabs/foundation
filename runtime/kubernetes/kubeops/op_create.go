// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeparser"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerCreate() {
	ops.RegisterVFuncs(ops.VFuncs[*kubedef.OpCreate, *parsedCreate]{
		Parse: func(ctx context.Context, def *fnschema.SerializedInvocation, create *kubedef.OpCreate) (*parsedCreate, error) {
			if create.BodyJson == "" {
				return nil, fnerrors.InternalError("create.Body is required")
			}

			parsed := &parsedCreate{spec: create}

			// XXX handle old versions of the secrets prebuilt which don't return a Kind within the body.
			if create.Resource == "secrets" {
				var obj kubeparser.ObjHeader
				if err := json.Unmarshal([]byte(create.BodyJson), &obj); err != nil {
					return nil, fnerrors.BadInputError("%s: kubernetes.create: failed to parse resource: %w", def.Description, err)
				}

				obj.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})

				parsed.obj = obj
				parsed.Name = obj.Name
				parsed.Namespace = obj.Namespace
			} else {
				var u unstructured.Unstructured
				if err := json.Unmarshal([]byte(create.BodyJson), &u); err != nil {
					return nil, fnerrors.BadInputError("kubernetes.apply: failed to parse resource: %w", err)
				}

				parsed.obj = &u
				parsed.Name = u.GetName()
				parsed.Namespace = u.GetNamespace()
			}

			if parsed.Name == "" {
				return nil, fnerrors.InternalError("%s: create.Name is required", def.Description)
			}

			return parsed, nil
		},

		Handle: func(ctx context.Context, d *fnschema.SerializedInvocation, parsed *parsedCreate) (*ops.HandleResult, error) {
			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			resource, err := resolveResource(ctx, cluster, parsed.obj.GetObjectKind().GroupVersionKind())
			if err != nil {
				return nil, err
			}

			restcfg := cluster.PreparedClient().RESTConfig

			create := parsed.spec
			if create.SkipIfAlreadyExists || create.UpdateIfExisting {
				obj, err := fetchResource(ctx, cluster, d.Description, *resource, parsed.Name,
					parsed.Namespace, fnschema.PackageNames(d.Scope...))
				if err != nil {
					return nil, fnerrors.New("failed to fetch resource: %w", err)
				}

				if obj != nil {
					if create.SkipIfAlreadyExists {
						return nil, nil // Nothing to do.
					}

					if create.UpdateIfExisting {
						msg := &unstructured.Unstructured{Object: map[string]interface{}{}}
						if err := msg.UnmarshalJSON([]byte(create.BodyJson)); err != nil {
							return nil, fnerrors.New("failed to parse create body: %w", err)
						}

						// This is not advised. Overwriting without reading.
						msg.SetResourceVersion(obj.GetResourceVersion())

						return nil, updateResource(ctx, d, *resource, msg, restcfg)
					}
				}
			}

			if err := tasks.Action("kubernetes.create").Scope(fnschema.PackageNames(d.Scope...)...).
				HumanReadablef(d.Description).
				Arg("resource", resource.Resource).
				Arg("name", parsed.Name).
				Arg("namespace", parsed.Namespace).Run(ctx, func(ctx context.Context) error {
				client, err := client.MakeGroupVersionBasedClient(ctx, restcfg, resource.GroupVersion())
				if err != nil {
					return err
				}

				req := client.Post()
				opts := metav1.CreateOptions{
					FieldManager: kubedef.K8sFieldManager,
				}

				if parsed.Namespace != "" {
					req.Namespace(parsed.Namespace)
				}

				r := req.Resource(resource.Resource).
					VersionedParams(&opts, metav1.ParameterCodec).
					Body([]byte(create.BodyJson))

				if OutputKubeApiURLs {
					fmt.Fprintf(console.Debug(ctx), "kubernetes: api post call %q\n", r.URL())
				}

				if err := r.Do(ctx).Error(); err != nil {
					return err
				}

				return nil
			}); err != nil {
				if !errors.IsNotFound(err) {
					return nil, fnerrors.InvocationError("%s: failed to create: %w", d.Description, err)
				}
			}

			if err := checkResetCRDCache(ctx, cluster, parsed.obj.GetObjectKind().GroupVersionKind()); err != nil {
				return nil, err
			}

			return nil, nil
		},

		PlanOrder: func(create *parsedCreate) (*fnschema.ScheduleOrder, error) {
			return kubedef.PlanOrder(create.obj.GetObjectKind().GroupVersionKind()), nil
		},
	})
}

func checkResetCRDCache(ctx context.Context, cluster runtime.Cluster, gvk schema.GroupVersionKind) error {
	if kubedef.IsGVKCRD(gvk) {
		mapper, err := cluster.EnsureState(ctx, kubernetes.RestmapperStateKey)
		if err != nil {
			return err
		}

		// Reset the cache after we install a new CRD.
		mapper.(*restmapper.DeferredDiscoveryRESTMapper).Reset()
	}

	return nil
}

type parsedCreate struct {
	obj interface {
		GetObjectKind() schema.ObjectKind
	}
	spec *kubedef.OpCreate

	Name, Namespace string
}

func updateResource(ctx context.Context, d *fnschema.SerializedInvocation, resource schema.GroupVersionResource, body *unstructured.Unstructured, restcfg *rest.Config) error {
	return tasks.Action("kubernetes.update").Scope(fnschema.PackageNames(d.Scope...)...).
		HumanReadablef(d.Description).
		Arg("resource", resource.Resource).
		Arg("name", body.GetName()).
		Arg("namespace", body.GetNamespace()).Run(ctx, func(ctx context.Context) error {
		client, err := client.MakeGroupVersionBasedClient(ctx, restcfg, resource.GroupVersion())
		if err != nil {
			return err
		}

		req := client.Put()
		opts := metav1.UpdateOptions{
			FieldManager: kubedef.K8sFieldManager,
		}

		if body.GetNamespace() != "" {
			req.Namespace(body.GetNamespace())
		}

		r := req.Resource(resource.Resource).
			Name(body.GetName()).
			VersionedParams(&opts, metav1.ParameterCodec).
			Body(body)

		if OutputKubeApiURLs {
			fmt.Fprintf(console.Debug(ctx), "kubernetes: api put call %q\n", r.URL())
		}

		return r.Do(ctx).Error()
	})
}
