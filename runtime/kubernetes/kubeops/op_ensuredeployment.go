// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerEnsureDeployment() {
	ops.RegisterVFuncs(ops.VFuncs[*kubedef.OpEnsureDeployment, *parsedEnsureDeployment]{
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

		Handle: func(ctx context.Context, d *fnschema.SerializedInvocation, parsed *parsedEnsureDeployment) (*ops.HandleResult, error) {
			return tasks.Return(ctx, tasks.Action("kubernetes.ensure-deployment").Scope(fnschema.PackageName(parsed.spec.Deployable.PackageName)),
				func(ctx context.Context) (*ops.HandleResult, error) {
					if parsed.spec.ConfigurationVolumeName == "" && len(parsed.spec.SetContainerField) == 0 {
						return apply(ctx, d.Description, fnschema.PackageNames(d.Scope...), parsed.obj, &kubedef.OpApply{
							BodyJson:      parsed.spec.SerializedResource,
							InhibitEvents: parsed.spec.InhibitEvents,
						})
					}

					inputs, err := ops.Get(ctx, ops.InputsInjection)
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
					})
				})
		},

		PlanOrder: func(ensure *parsedEnsureDeployment) (*fnschema.ScheduleOrder, error) {
			return kubedef.PlanOrder(ensure.obj.GroupVersionKind()), nil
		},
	})
}

func patchObject(obj kubedef.Object, spec *kubedef.OpEnsureDeployment, output *kubedef.EnsureRuntimeConfigOutput, setFields []*runtime.SetContainerField) (any, error) {
	switch {
	case kubedef.IsDeployment(obj):
		var d specOnlyDeployment
		if err := json.Unmarshal([]byte(spec.SerializedResource), &d); err != nil {
			return nil, err
		}

		patchConfigID(&d.ObjectMeta, &d.Spec.Template.Spec, output.ConfigId, spec.ConfigurationVolumeName)
		patchSetFields(&d.ObjectMeta, &d.Spec.Template.Spec, setFields, output)
		return &d, nil

	case kubedef.IsStatefulSet(obj):
		var d specOnlyStatefulSet
		if err := json.Unmarshal([]byte(spec.SerializedResource), &d); err != nil {
			return nil, err
		}

		patchConfigID(&d.ObjectMeta, &d.Spec.Template.Spec, output.ConfigId, spec.ConfigurationVolumeName)
		patchSetFields(&d.ObjectMeta, &d.Spec.Template.Spec, setFields, output)
		return &d, nil

	case kubedef.IsPod(obj):
		var d specOnlyPod
		if err := json.Unmarshal([]byte(spec.SerializedResource), &d); err != nil {
			return nil, err
		}

		patchConfigID(&d.ObjectMeta, &d.Spec, output.ConfigId, spec.ConfigurationVolumeName)
		patchSetFields(&d.ObjectMeta, &d.Spec, setFields, output)
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

func patchSetFields(metadata *metav1.ObjectMeta, spec *v1.PodSpec, setFields []*runtime.SetContainerField, output *kubedef.EnsureRuntimeConfigOutput) error {
	var errs []error
	for _, setField := range setFields {
		for _, setArg := range setField.GetSetArg() {
			value, err := selectValue(output, setArg.Value)
			if err != nil {
				errs = append(errs, err)
			} else {
				errs = append(errs, updateContainers(spec, setArg.ContainerName, func(container *v1.Container) {
					container.Args = append(container.Args, setArg.Key+"="+value)
				}))
			}
		}

		for _, setEnv := range setField.GetSetEnv() {
			value, err := selectValue(output, setEnv.Value)
			if err != nil {
				errs = append(errs, err)
			} else {
				errs = append(errs, updateContainers(spec, setEnv.ContainerName, func(container *v1.Container) {
					container.Env = append(container.Env, v1.EnvVar{Name: setEnv.Key, Value: value})
				}))
			}
		}
	}
	return multierr.New(errs...)
}

func selectValue(output *kubedef.EnsureRuntimeConfigOutput, source runtime.SetContainerField_ValueSource) (string, error) {
	switch source {
	case runtime.SetContainerField_RUNTIME_CONFIG:
		return output.SerializedRuntimeJson, nil

	case runtime.SetContainerField_RESOURCE_CONFIG:
		return output.SerializedResourceJson, nil
	}

	return "", fnerrors.BadInputError("%s: don't know this value", source)
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
