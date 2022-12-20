// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeops

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/encoding/protojson"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	fnschema "namespacelabs.dev/foundation/schema"
	orchpb "namespacelabs.dev/foundation/schema/orchestration"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

func registerEnsureDeployment() {
	execution.RegisterVFuncs(execution.VFuncs[*kubedef.OpEnsureDeployment, *parsedEnsureDeployment]{
		Parse: func(ctx context.Context, def *fnschema.SerializedInvocation, ensure *kubedef.OpEnsureDeployment) (*parsedEnsureDeployment, error) {
			if ensure.SerializedResource == "" {
				return nil, fnerrors.InternalError("EnsureDeployment.SerializedResource is required")
			}

			var parsed unstructured.Unstructured
			if err := json.Unmarshal([]byte(ensure.SerializedResource), &parsed); err != nil {
				return nil, fnerrors.BadInputError("kubernetes.ensuredeployment: failed to parse resource: %w", err)
			}

			return &parsedEnsureDeployment{obj: &parsed, spec: ensure}, nil
		},

		EmitStart: func(ctx context.Context, d *fnschema.SerializedInvocation, parsed *parsedEnsureDeployment, ch chan *orchpb.Event) {
			if parsed.spec.InhibitEvents {
				return
			}

			ev := kubeobserver.PrepareEvent(parsed.obj.GroupVersionKind(), parsed.obj.GetNamespace(), parsed.obj.GetName(), d.Description, parsed.spec.Deployable)
			ev.Stage = orchpb.Event_WAITING
			ch <- ev
		},

		HandleWithEvents: func(ctx context.Context, d *fnschema.SerializedInvocation, parsed *parsedEnsureDeployment, ch chan *orchpb.Event) (*execution.HandleResult, error) {
			return tasks.Return(ctx, tasks.Action("kubernetes.ensure-deployment").Scope(parsed.spec.Deployable.GetPackageRef().AsPackageName()),
				func(ctx context.Context) (*execution.HandleResult, error) {
					if parsed.spec.ConfigurationVolumeName == "" && len(parsed.spec.SetContainerField) == 0 {
						return apply(ctx, d.Description, fnschema.PackageNames(d.Scope...), parsed.obj, &kubedef.OpApply{
							BodyJson:      parsed.spec.SerializedResource,
							InhibitEvents: parsed.spec.InhibitEvents,
						}, ch)
					}

					inputs, err := execution.Get(ctx, execution.InputsInjection)
					if err != nil {
						return nil, err
					}

					outputMsg, ok := inputs[kubedef.RuntimeConfigOutput(parsed.spec.Deployable)]
					if !ok {
						return nil, fnerrors.InternalError("%s: input missing", kubedef.RuntimeConfigOutput(parsed.spec.Deployable))
					}

					output := outputMsg.Message.(*kubedef.EnsureRuntimeConfigOutput)

					renewed, err := patchObject(parsed.obj, parsed.spec, output, parsed.spec.SetContainerField)
					if err != nil {
						return nil, err
					}

					serializedRenewed, err := json.Marshal(renewed)
					if err != nil {
						return nil, fnerrors.InternalError("failed to serialize deployment: %w", err)
					}

					var reparsed unstructured.Unstructured
					if err := json.Unmarshal(serializedRenewed, &reparsed); err != nil {
						return nil, fnerrors.InternalError("failed to reparse deployment: %w", err)
					}

					return apply(ctx, d.Description, fnschema.PackageNames(d.Scope...), &reparsed, &kubedef.OpApply{
						BodyJson:      string(serializedRenewed),
						InhibitEvents: parsed.spec.InhibitEvents,
						Deployable:    parsed.spec.Deployable,
					}, ch)
				})
		},

		PlanOrder: func(ctx context.Context, ensure *parsedEnsureDeployment) (*fnschema.ScheduleOrder, error) {
			return kubedef.PlanOrder(ensure.obj.GroupVersionKind(), ensure.obj.GetNamespace(), ensure.obj.GetName()), nil
		},
	})
}

func patchObject(obj kubedef.Object, spec *kubedef.OpEnsureDeployment, output *kubedef.EnsureRuntimeConfigOutput, setFields []*runtimepb.SetContainerField) (any, error) {
	switch {
	case kubedef.IsDeployment(obj):
		var d specOnlyDeployment
		if err := json.Unmarshal([]byte(spec.SerializedResource), &d); err != nil {
			return nil, err
		}

		patchConfigID(&d.ObjectMeta, &d.Spec.Template.Spec, output.ConfigId, spec.ConfigurationVolumeName)
		if err := patchSetFields(&d.ObjectMeta, &d.Spec.Template.Spec, setFields, output); err != nil {
			return nil, err
		}
		return &d, nil

	case kubedef.IsStatefulSet(obj):
		var d specOnlyStatefulSet
		if err := json.Unmarshal([]byte(spec.SerializedResource), &d); err != nil {
			return nil, err
		}

		patchConfigID(&d.ObjectMeta, &d.Spec.Template.Spec, output.ConfigId, spec.ConfigurationVolumeName)
		if err := patchSetFields(&d.ObjectMeta, &d.Spec.Template.Spec, setFields, output); err != nil {
			return nil, err
		}
		return &d, nil

	case kubedef.IsDaemonSet(obj):
		var d specOnlyDaemonSet
		if err := json.Unmarshal([]byte(spec.SerializedResource), &d); err != nil {
			return nil, err
		}

		patchConfigID(&d.ObjectMeta, &d.Spec.Template.Spec, output.ConfigId, spec.ConfigurationVolumeName)
		if err := patchSetFields(&d.ObjectMeta, &d.Spec.Template.Spec, setFields, output); err != nil {
			return nil, err
		}
		return &d, nil

	case kubedef.IsPod(obj):
		var d specOnlyPod
		if err := json.Unmarshal([]byte(spec.SerializedResource), &d); err != nil {
			return nil, err
		}

		patchConfigID(&d.ObjectMeta, &d.Spec, output.ConfigId, spec.ConfigurationVolumeName)
		if err := patchSetFields(&d.ObjectMeta, &d.Spec, setFields, output); err != nil {
			return nil, err
		}
		return &d, nil

	default:
		return nil, fnerrors.InternalError("unsupported deployment kind")
	}
}

type specOnlyDeployment struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the Deployment.
	// +optional
	Spec appsv1.DeploymentSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type specOnlyStatefulSet struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired identities of pods in this set.
	// +optional
	Spec appsv1.StatefulSetSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type specOnlyDaemonSet struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired identities of pods in this set.
	// +optional
	Spec appsv1.DaemonSetSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type specOnlyPod struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec v1.PodSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

func patchConfigID(metadata *metav1.ObjectMeta, spec *v1.PodSpec, configID, configVolumeName string) {
	if configID == "" {
		return
	}

	// We do manual cleanup of unused configs. In the future they'll be owned by a deployment intent.
	metadata.Annotations[kubedef.K8sRuntimeConfig] = configID

	spec.Volumes = append(spec.Volumes, v1.Volume{
		Name: configVolumeName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{Name: configID},
			},
		},
	})
}

func patchSetFields(metadata *metav1.ObjectMeta, spec *v1.PodSpec, setFields []*runtimepb.SetContainerField, output *kubedef.EnsureRuntimeConfigOutput) error {
	var errs []error
	for _, setField := range setFields {
		for _, setArg := range setField.GetSetArg() {
			value, err := selectValue(output, setArg)
			if err != nil {
				errs = append(errs, err)
			} else {
				if value.From != nil {
					errs = append(errs, fnerrors.BadInputError("%s: can't set an argument from an env source", setArg.Key))
				} else {
					errs = append(errs, updateContainers(spec, setArg.ContainerName, func(container *v1.Container) {
						container.Args = append(container.Args, setArg.Key+"="+value.Inline)
					}))
				}
			}
		}

		for _, setEnv := range setField.GetSetEnv() {
			value, err := selectValue(output, setEnv)
			if err != nil {
				errs = append(errs, err)
			} else {
				errs = append(errs, updateContainers(spec, setEnv.ContainerName, func(container *v1.Container) {
					container.Env = append(container.Env, v1.EnvVar{Name: setEnv.Key, Value: value.Inline, ValueFrom: value.From})
				}))
			}
		}
	}
	return multierr.New(errs...)
}

type value struct {
	Inline string
	From   *v1.EnvVarSource
}

func selectValue(output *kubedef.EnsureRuntimeConfigOutput, set *runtimepb.SetContainerField_SetValue) (*value, error) {
	switch set.Value {
	case runtimepb.SetContainerField_RUNTIME_CONFIG:
		return &value{Inline: output.SerializedRuntimeJson}, nil

	case runtimepb.SetContainerField_RESOURCE_CONFIG:
		return &value{Inline: output.SerializedResourceJson}, nil

	case runtimepb.SetContainerField_RUNTIME_CONFIG_SERVICE_ENDPOINT:
		endpoint, err := selectServiceValue(set.ServiceRef, output.SerializedRuntimeJson, runtime.SelectServiceEndpoint)
		if err != nil {
			return nil, err
		}

		// Returns a hostname:port pair.
		return &value{Inline: endpoint}, nil

	case runtimepb.SetContainerField_RUNTIME_CONFIG_SERVICE_INGRESS_BASE_URL:
		ingressUrl, err := selectServiceValue(set.ServiceRef, output.SerializedRuntimeJson, runtime.SelectServiceIngress)
		if err != nil {
			return nil, err
		}

		return &value{Inline: ingressUrl}, nil

	case runtimepb.SetContainerField_RESOURCE_CONFIG_FIELD_SELECTOR:
		if set.ResourceConfigFieldSelector == nil {
			return nil, fnerrors.BadInputError("missing required field selector")
		}

		resources, err := resources.ParseResourceData([]byte(output.SerializedResourceJson))
		if err != nil {
			return nil, fnerrors.InternalError("failed to unmarshal resource configuration: %w", err)
		}

		v, err := resources.SelectField(set.ResourceConfigFieldSelector.GetResource().Canonical(), set.ResourceConfigFieldSelector.GetFieldSelector())
		if err != nil {
			return nil, fnerrors.InternalError("failed to select resource value: %w", err)
		}

		switch x := v.(type) {
		case string:
			return &value{Inline: x}, nil

		case int32, int64, uint32, uint64, int:
			return &value{Inline: fmt.Sprintf("%d", x)}, nil

		default:
			return nil, fnerrors.BadInputError("unsupported resource field value %q", reflect.TypeOf(v).String())
		}
	}

	return nil, fnerrors.BadInputError("%s: don't know this value", set.Value)
}

func selectServiceValue(ref *fnschema.ServiceRef, serializedResourceJson string, selector func(*runtimepb.Server_Service) (string, error)) (string, error) {
	if ref == nil {
		return "", fnerrors.BadInputError("missing required service endpoint")
	}

	rt := &runtimepb.RuntimeConfig{}
	// XXX unmarshal once.
	if err := protojson.Unmarshal([]byte(serializedResourceJson), rt); err != nil {
		return "", fnerrors.InternalError("failed to unmarshal runtime configuration: %w", err)
	}

	return runtime.SelectServiceValue(rt, ref, selector)
}

func updateContainers(spec *v1.PodSpec, name string, update func(container *v1.Container)) error {
	count := 0
	for k, container := range spec.Containers {
		if name != "" && container.Name != name {
			continue
		}

		c := container
		update(&c)
		spec.Containers[k] = c
		count++
	}

	if name != "" && count == 0 {
		return fnerrors.BadInputError("no container matched name %q", name)
	}

	return nil
}

type parsedEnsureDeployment struct {
	obj  *unstructured.Unstructured
	spec *kubedef.OpEnsureDeployment
}
