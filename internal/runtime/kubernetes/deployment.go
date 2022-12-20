// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/internal/support"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/library/kubernetes/rbac"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution/defs"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/foundation/std/runtime/constants"
	"namespacelabs.dev/go-ids"
	"sigs.k8s.io/yaml"
)

//go:embed defaults/*.yaml
var defaults embed.FS

var DeployAsPodsInTests = true

const (
	kubeNode schema.PackageName = "namespacelabs.dev/foundation/std/runtime/kubernetes"

	revisionHistoryLimit int32 = 10
)

type perEnvConf struct {
	dashnessPeriod        int32
	livenessInitialDelay  int32
	readinessInitialDelay int32
	probeTimeout          int32
	failureThreshold      int32
}

var perEnvConfMapping = map[schema.Environment_Purpose]*perEnvConf{
	schema.Environment_DEVELOPMENT: {
		dashnessPeriod:        1,
		livenessInitialDelay:  1,
		readinessInitialDelay: 1,
		probeTimeout:          1,
		failureThreshold:      3,
	},
	schema.Environment_TESTING: {
		dashnessPeriod:        1,
		livenessInitialDelay:  1,
		readinessInitialDelay: 1,
		probeTimeout:          1,
		failureThreshold:      3,
	},
	schema.Environment_PRODUCTION: {
		dashnessPeriod:        3,
		livenessInitialDelay:  1,
		readinessInitialDelay: 3,
		probeTimeout:          1,
		failureThreshold:      5,
	},
}

type definitions []definition

type definition interface {
	defs.MakeDefinition
	AppliedResource() any
}

type serverRunState struct {
	operations definitions
}

func getArg(c *applycorev1.ContainerApplyConfiguration, name string) (string, bool) {
	for _, arg := range c.Args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		// Remove up to two dashes.
		cleaned := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
		parts := strings.SplitN(cleaned, "=", 2)
		if len(parts) != 2 {
			continue
		}

		if parts[0] == name {
			return parts[1], true
		}
	}

	return "", false
}

func toK8sProbe(p *applycorev1.ProbeApplyConfiguration, probevalues *perEnvConf, probe *schema.Probe) (*applycorev1.ProbeApplyConfiguration, error) {
	p = p.WithPeriodSeconds(probevalues.dashnessPeriod).
		WithFailureThreshold(probevalues.failureThreshold).
		WithTimeoutSeconds(probevalues.probeTimeout)

	switch {
	case probe.Http != nil:
		return p.WithHTTPGet(applycorev1.HTTPGetAction().WithPath(probe.Http.GetPath()).
			WithPort(intstr.FromInt(int(probe.Http.GetContainerPort())))), nil

	case probe.Exec != nil:
		return p.WithExec(applycorev1.ExecAction().WithCommand(probe.Exec.Command...)), nil

	default:
		return nil, fnerrors.InternalError("unknown probe type for %q", probe.Kind)
	}
}

type deployOpts struct {
	secrets secrets.GroundedSecrets
}

func deployAsPods(env *schema.Environment) bool {
	return env.GetPurpose() == schema.Environment_TESTING && DeployAsPodsInTests
}

// Transient data structure used to prepare volumes and mounts
type volumeDef struct {
	name string
	// True if the volume is actually a filesync and needs a different handling.
	isWorkspaceSync bool
}

func prepareDeployment(ctx context.Context, target clusterTarget, deployable runtime.DeployableSpec, opts deployOpts, s *serverRunState) error {
	if deployable.MainContainer.Image.Repository == "" {
		return fnerrors.InternalError("kubernetes: no repository defined in image: %v", deployable.MainContainer.Image)
	}

	secCtx, err := makeSecurityContext(deployable.MainContainer)
	if err != nil {
		return err
	}

	name := kubedef.ServerCtrName(deployable)
	containers := []string{name}
	mainContainer := applycorev1.Container().
		WithName(name).
		WithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError).
		WithImage(deployable.MainContainer.Image.RepoAndDigest()).
		WithArgs(deployable.MainContainer.Args...).
		WithCommand(deployable.MainContainer.Command...).
		WithSecurityContext(secCtx)

	for _, port := range deployable.MainContainer.AllocatedPorts {
		mainContainer.WithPorts(applycorev1.ContainerPort().WithName(port.Name).WithContainerPort(port.ContainerPort))
	}

	switch deployable.Attachable {
	case runtime.AttachableKind_WITH_STDIN_ONLY:
		mainContainer = mainContainer.WithStdin(true).WithStdinOnce(true)

	case runtime.AttachableKind_WITH_TTY:
		mainContainer = mainContainer.WithStdin(true).WithStdinOnce(true).WithTTY(true)
	}

	mainEnv := slices.Clone(deployable.MainContainer.Env)

	if deployable.MainContainer.WorkingDir != "" {
		mainContainer = mainContainer.WithWorkingDir(deployable.MainContainer.WorkingDir)
	}

	spec := applycorev1.PodSpec().
		WithEnableServiceLinks(false) // Disable service injection via environment variables.

	labels := kubedef.MakeLabels(target.env, deployable)
	annotations := kubedef.MakeAnnotations(target.env, deployable.GetPackageRef().AsPackageName())
	deploymentId := kubedef.MakeDeploymentId(deployable)

	tmpl := applycorev1.PodTemplateSpec().
		WithAnnotations(annotations).
		WithLabels(labels)

	var initVolumeMounts []*applycorev1.VolumeMountApplyConfiguration
	// Key: PackageRef.CanonicalString().
	initArgs := map[string][]string{}

	var serviceAccount string // May be specified by a SpecExtension.
	var createServiceAccount bool
	var serviceAccountAnnotations []*kubedef.SpecExtension_Annotation

	if len(deployable.Permissions.GetClusterRole()) > 0 {
		createServiceAccount = true
		serviceAccount = kubedef.MakeDeploymentId(deployable)

		for _, role := range deployable.Permissions.ClusterRole {
			var clusterRole *runtime.PlannedResource
			for _, res := range deployable.PlannedResources {
				if res.ResourceInstanceID == role.ResourceId {
					clusterRole = &res
					// XXX check dups?
				}
			}

			if clusterRole == nil {
				return fnerrors.New("permissions refer to %q which is not present", role.Label)
			}

			const roleInstance = "library.kubernetes.rbac.ClusterRoleInstance"
			if clusterRole.Class.GetInstanceType().GetProtoType() != roleInstance {
				return fnerrors.New("expected resource class to be %q, got %q", roleInstance, clusterRole.Class.GetInstanceType().GetProtoType())
			}

			instance := &rbac.ClusterRoleInstance{}
			if err := proto.Unmarshal(clusterRole.Instance.Value, instance); err != nil {
				return fnerrors.New("failed to unmarshal instance %q: %w", roleInstance, err)
			}

			s.operations = append(s.operations, kubedef.Apply{
				Description: fmt.Sprintf("%s: bind Cluster Role", role.Label),
				Resource: rbacv1.ClusterRoleBinding(fmt.Sprintf("binding:%s:%s", kubedef.MakeDeploymentId(deployable), instance.Name)).
					WithLabels(labels).
					WithAnnotations(kubedef.BaseAnnotations()).
					WithRoleRef(rbacv1.RoleRef().
						WithAPIGroup("rbac.authorization.k8s.io").
						WithKind("ClusterRole").
						WithName(instance.Name)).
					WithSubjects(rbacv1.Subject().
						WithKind("ServiceAccount").
						WithNamespace(target.namespace).
						WithName(serviceAccount)),
			})
		}
	}

	var specifiedSec *kubedef.SpecExtension_SecurityContext

	probes := deployable.Probes
	for _, input := range deployable.Extensions {
		specExt := &kubedef.SpecExtension{}
		containerExt := &kubedef.ContainerExtension{}
		initContainerExt := &kubedef.InitContainerExtension{}

		switch {
		case input.Impl.MessageIs(specExt):
			if err := input.Impl.UnmarshalTo(specExt); err != nil {
				return fnerrors.InternalError("failed to unmarshal SpecExtension: %w", err)
			}

			for _, vol := range specExt.Volume {
				k8svol, err := toK8sVol(vol)
				if err != nil {
					return err
				}
				spec = spec.WithVolumes(k8svol)
			}

			if len(specExt.Annotation) > 0 {
				m := map[string]string{}
				for _, annotation := range specExt.Annotation {
					m[annotation.Key] = annotation.Value
				}
				tmpl = tmpl.WithAnnotations(m)
			}

			if specExt.EnsureServiceAccount {
				createServiceAccount = true
				if specExt.ServiceAccount == "" {
					return fnerrors.NewWithLocation(deployable.ErrorLocation, "ensure_service_account requires service_account to be set")
				}
			}

			serviceAccountAnnotations = append(serviceAccountAnnotations, specExt.ServiceAccountAnnotation...)

			if specExt.ServiceAccount != "" {
				if serviceAccount != "" && serviceAccount != specExt.ServiceAccount {
					return fnerrors.NewWithLocation(deployable.ErrorLocation, "incompatible service accounts defined, %q vs %q",
						serviceAccount, specExt.ServiceAccount)
				}
				serviceAccount = specExt.ServiceAccount
			}

			if specExt.SecurityContext != nil {
				if !protos.CheckConsolidate(specExt.SecurityContext, &specifiedSec) {
					return fnerrors.NewWithLocation(deployable.ErrorLocation, "incompatible securitycontext defined, %v vs %v",
						specifiedSec, specExt.SecurityContext)
				}
			}

		case input.Impl.MessageIs(containerExt):
			if err := input.Impl.UnmarshalTo(containerExt); err != nil {
				return fnerrors.InternalError("failed to unmarshal ContainerExtension: %w", err)
			}

			for _, mount := range containerExt.VolumeMount {
				volumeMount := applycorev1.VolumeMount().
					WithName(mount.Name).
					WithReadOnly(mount.ReadOnly).
					WithMountPath(mount.MountPath)
				mainContainer = mainContainer.WithVolumeMounts(volumeMount)
				if mount.MountOnInit {
					// Volume mounts may declare to be available also during server initialization.
					// E.g. Initializing the schema of a data store requires early access to server secrets.
					// The volume mount provider has full control over whether the volume is available.
					initVolumeMounts = append(initVolumeMounts, volumeMount)
				}
			}

			var err error
			mainEnv, err = support.MergeEnvs(mainEnv, containerExt.Env)
			if err != nil {
				return err
			}

			if containerExt.Args != nil {
				mainContainer = mainContainer.WithArgs(containerExt.Args...)
			} else {
				// Deprecated path.
				for _, arg := range containerExt.ArgTuple {
					if currentValue, found := getArg(mainContainer, arg.Name); found && currentValue != arg.Value {
						return fnerrors.NewWithLocation(deployable.ErrorLocation, "argument '%s' is already set to '%s' but would be overwritten to '%s' by container extension", arg.Name, currentValue, arg.Value)
					}
					mainContainer = mainContainer.WithArgs(fmt.Sprintf("--%s=%s", arg.Name, arg.Value))
				}
			}

			probes = append(probes, containerExt.Probe...)

			// Deprecated path.
			for _, initContainer := range containerExt.InitContainer {
				key := initContainer.PackageName
				initArgs[key] = append(initArgs[key], initContainer.Arg...)
			}

		case input.Impl.MessageIs(initContainerExt):
			if err := input.Impl.UnmarshalTo(initContainerExt); err != nil {
				return fnerrors.InternalError("failed to unmarshal InitContainerExtension: %w", err)
			}

			var key string
			if initContainerExt.PackageRef != nil {
				key = initContainerExt.PackageRef.Canonical()
			} else {
				key = initContainerExt.PackageName
			}
			initArgs[key] = append(initArgs[key], initContainerExt.Args...)

		default:
			return fnerrors.InternalError("unused startup input: %s", input.Impl.GetTypeUrl())
		}
	}

	var readinessProbe, livenessProbe *schema.Probe
	for _, probe := range probes {
		switch probe.Kind {
		case runtime.FnServiceLivez:
			if !protos.CheckConsolidate(probe, &livenessProbe) {
				return fnerrors.BadInputError("inconsistent live probe definition")
			}
		case runtime.FnServiceReadyz:
			if !protos.CheckConsolidate(probe, &readinessProbe) {
				return fnerrors.BadInputError("inconsistent ready probe definition")
			}
		default:
			return fnerrors.BadInputError("%s: unknown probe kind", probe.Kind)
		}
	}

	probevalues := perEnvConfMapping[target.env.GetPurpose()]
	if readinessProbe != nil || livenessProbe != nil {
		if probevalues == nil {
			return fnerrors.InternalError("%s: no constants configured", target.env.GetPurpose())
		}
	}

	if readinessProbe != nil {
		probe, err := toK8sProbe(applycorev1.Probe().WithInitialDelaySeconds(probevalues.readinessInitialDelay),
			probevalues, readinessProbe)
		if err != nil {
			return err
		}
		mainContainer = mainContainer.WithReadinessProbe(probe)
	}

	if livenessProbe != nil {
		probe, err := toK8sProbe(applycorev1.Probe().WithInitialDelaySeconds(probevalues.livenessInitialDelay),
			probevalues, livenessProbe)
		if err != nil {
			return err
		}
		mainContainer = mainContainer.WithLivenessProbe(probe)
	}

	// XXX Think through this, should it also be hashed + immutable?
	secretId := fmt.Sprintf("ns-managed-%s-%s", deployable.Name, deployable.GetId())
	seccol := newSecretCollector(secretId)

	ensure := kubedef.EnsureDeployment{
		Deployable:    deployable,
		InhibitEvents: deployable.Class == schema.DeployableClass_MANUAL || (target.namespace == kubedef.AdminNamespace && !deployable.Focused),
		SchedCategory: []string{
			runtime.DeployableCategory(deployable),
			runtime.OwnedByDeployable(deployable),
		},
		SetContainerFields: slices.Clone(deployable.SetContainerField),
	}

	if _, err := fillEnv(ctx, deployable.RuntimeConfig, mainContainer, mainEnv, opts.secrets, seccol, &ensure); err != nil {
		return err
	}

	volumes := deployable.Volumes
	mounts := deployable.MainContainer.Mounts

	volumeDefs := map[string]*volumeDef{}
	for k, volume := range volumes {
		if volume.Name == "" {
			return fnerrors.InternalError("volume #%d is missing a name", k)
		}

		name := kubedef.MakeVolumeName(volume)
		volumeDef := &volumeDef{name: name}
		volumeDefs[volume.Name] = volumeDef

		switch volume.Kind {
		case constants.VolumeKindEphemeral:
			spec = spec.WithVolumes(applycorev1.Volume().WithName(name).WithEmptyDir(applycorev1.EmptyDirVolumeSource()))

		case constants.VolumeKindHostPath:
			pv := &schema.HostPathVolume{}
			if err := volume.Definition.UnmarshalTo(pv); err != nil {
				return fnerrors.InternalError("%s: failed to unmarshal hostDir volume definition: %w", volume.Name, err)
			}

			switch {
			case pv.Directory != "":
				spec = spec.WithVolumes(applycorev1.Volume().WithName(name).
					WithHostPath(applycorev1.HostPathVolumeSource().
						WithType(v1.HostPathDirectoryOrCreate).WithPath(pv.Directory)))
			}

		case constants.VolumeKindPersistent:
			pv := &schema.PersistentVolume{}
			if err := volume.Definition.UnmarshalTo(pv); err != nil {
				return fnerrors.InternalError("%s: failed to unmarshal persistent volume definition: %w", volume.Name, err)
			}

			if pv.Id == "" {
				return fnerrors.BadInputError("%s: persistent ID is missing", volume.Name)
			}

			v, operations, err := makePersistentVolume(target.namespace, target.env, deployable.ErrorLocation, volume.Owner, name, pv.Id, pv.SizeBytes, annotations)
			if err != nil {
				return err
			}

			spec = spec.WithVolumes(v)
			s.operations = append(s.operations, operations...)

		case constants.VolumeKindWorkspaceSync:
			volumeDef.isWorkspaceSync = true

		case constants.VolumeKindConfigurable:
			cv := &schema.ConfigurableVolume{}
			if err := volume.Definition.UnmarshalTo(cv); err != nil {
				return fnerrors.InternalError("%s: failed to unmarshal configurable volume definition: %w", volume.Name, err)
			}

			configs := newDataItemCollector()

			configHash := sha256.New()

			projected := applycorev1.ProjectedVolumeSource()

			var configmapItems []*applycorev1.KeyToPathApplyConfiguration
			var secretItems []*applycorev1.KeyToPathApplyConfiguration
			for _, entry := range cv.Entries {
				switch {
				case entry.Inline != nil:
					configmapItems = append(configmapItems, makeConfigEntry(configHash, entry, entry.Inline, configs).WithPath(entry.Path))

				case entry.InlineSet != nil:
					for _, rsc := range entry.InlineSet.Resource {
						configmapItems = append(configmapItems, makeConfigEntry(configHash, entry, rsc, configs).WithPath(filepath.Join(entry.Path, rsc.Path)))
					}

				case entry.SecretRef != nil:
					resource, err := opts.secrets.Get(ctx, entry.SecretRef)
					if err != nil {
						return err
					}

					if resource.Value != nil {
						secretItems = append(secretItems, makeConfigEntry(configHash, entry, resource.Value, seccol.items).WithPath(entry.Path))
					} else if resource.Spec.Generate != nil {
						name, key := seccol.allocateGenerated(resource.Ref, resource.Spec)
						projected = projected.WithSources(applycorev1.VolumeProjection().WithSecret(
							applycorev1.SecretProjection().WithName(name).WithItems(applycorev1.KeyToPath().WithKey(key).WithPath(entry.Path)),
						))
					}

				case entry.KubernetesSecretRef != nil:
					projected = projected.WithSources(applycorev1.VolumeProjection().WithSecret(
						applycorev1.SecretProjection().WithName(entry.KubernetesSecretRef.SecretName).
							WithItems(applycorev1.KeyToPath().WithKey(entry.KubernetesSecretRef.SecretName).WithPath(entry.Path)),
					))
				}
			}

			if len(configmapItems) > 0 {
				configId := "ns-static-" + ids.EncodeToBase32String(configHash.Sum(nil))[6:]

				projected = projected.WithSources(applycorev1.VolumeProjection().WithConfigMap(
					applycorev1.ConfigMapProjection().WithName(configId).WithItems(configmapItems...)))

				// Needs to be declared before it's used.
				s.operations = append(s.operations, kubedef.Apply{
					Description: "Static configuration",
					Resource: applycorev1.ConfigMap(configId, target.namespace).
						WithAnnotations(annotations).
						WithLabels(labels).
						WithLabels(map[string]string{
							kubedef.K8sKind: kubedef.K8sStaticConfigKind,
						}).
						WithImmutable(true).
						WithData(configs.data).
						WithBinaryData(configs.binaryData),
				})
			}

			if len(secretItems) > 0 {
				projected = projected.WithSources(applycorev1.VolumeProjection().WithSecret(
					applycorev1.SecretProjection().WithName(secretId).WithItems(secretItems...)))
			}

			spec = spec.WithVolumes(applycorev1.Volume().WithName(name).WithProjected(projected))

		default:
			return fnerrors.InternalError("%s: unsupported volume type", volume.Kind)
		}
	}

	for k, mount := range mounts {
		if mount.Path == "" {
			return fnerrors.InternalError("mount #%d is missing a path", k)
		}

		if mount.VolumeRef == nil {
			return fnerrors.InternalError("mount %q is missing a target volume", mount.Path)
		}

		volumeDef, ok := volumeDefs[mount.VolumeRef.Name]
		if !ok {
			return fnerrors.InternalError("unknown target volume %q for mount %q", mount.VolumeRef.Name, mount.Path)
		}

		if !volumeDef.isWorkspaceSync {
			mainContainer = mainContainer.WithVolumeMounts(applycorev1.VolumeMount().
				WithMountPath(mount.Path).
				WithName(volumeDef.name).
				WithReadOnly(mount.Readonly))
		}
	}

	regularResources := slices.Clone(deployable.ResourceDeps)

	const projectedSecretsVolName = "ns-projected-secrets"

	var secretProjections []*applycorev1.SecretProjectionApplyConfiguration
	var injected []*kubedef.OpEnsureRuntimeConfig_InjectedResource
	for _, res := range deployable.SecretResources {
		if res.Spec.Generate == nil {
			return fnerrors.BadInputError("don't yet support secrets used in resources, which don't use a generate block")
		}

		name, key := seccol.allocateGenerated(res.SecretRef, res.Spec)

		serialized, err := secrets.Serialize(res.ResourceRef)
		if err != nil {
			return err
		}

		secretProjections = append(secretProjections,
			applycorev1.SecretProjection().WithName(name).WithItems(
				applycorev1.KeyToPath().WithKey(key).WithPath(serialized.RelPath)))

		injected = append(injected, &kubedef.OpEnsureRuntimeConfig_InjectedResource{
			ResourceRef:    res.ResourceRef,
			SerializedJson: serialized.JSON,
		})
	}

	for _, dep := range deployable.PlannedResourceDeps {
		handled := false
		for _, x := range deployable.PlannedResources {
			if dep.ResourceInstanceId == x.ResourceInstanceID {
				injected = append(injected, &kubedef.OpEnsureRuntimeConfig_InjectedResource{
					ResourceRef:    dep.ResourceRef,
					SerializedJson: x.InstanceSerializedJSON,
				})
				handled = true
				break
			}
		}

		if !handled {
			return fnerrors.InternalError("%s: resource value is missing", dep.ResourceRef.Canonical())
		}
	}

	if len(secretProjections) > 0 {
		src := applycorev1.ProjectedVolumeSource()
		for _, p := range secretProjections {
			src = src.WithSources(applycorev1.VolumeProjection().WithSecret(p))
		}

		spec = spec.WithVolumes(applycorev1.Volume().WithName(projectedSecretsVolName).WithProjected(src))
		mainContainer = mainContainer.WithVolumeMounts(applycorev1.VolumeMount().
			WithName(projectedSecretsVolName).
			WithMountPath(secrets.SecretBaseMountPath).
			WithReadOnly(true))
	}

	// Before sidecars so they have access to the "runtime config" volume.
	if deployable.RuntimeConfig != nil || len(regularResources) > 0 || len(injected) > 0 {
		slices.SortFunc(regularResources, func(a, b *resources.ResourceDependency) bool {
			return strings.Compare(a.ResourceInstanceId, b.ResourceInstanceId) < 0
		})

		ensureConfig := kubedef.EnsureRuntimeConfig{
			Description:          "Runtime configuration",
			RuntimeConfig:        deployable.RuntimeConfig,
			Deployable:           deployable,
			ResourceDependencies: regularResources,
			InjectedResources:    injected,
			BuildVCS:             deployable.BuildVCS,
			PersistConfiguration: deployable.MountRuntimeConfigPath != "",
		}

		s.operations = append(s.operations, ensureConfig)

		// Make sure we wait for the runtime configuration to be created before
		// deploying a new deployment or statefulset.
		ensure.RuntimeConfigDependency = kubedef.RuntimeConfigOutput(deployable)

		if deployable.MountRuntimeConfigPath != "" {
			ensure.ConfigurationVolumeName = "namespace-rtconfig"
			mainContainer = mainContainer.WithVolumeMounts(
				applycorev1.VolumeMount().
					WithMountPath(deployable.MountRuntimeConfigPath).
					WithName(ensure.ConfigurationVolumeName).
					WithReadOnly(true))
		}
	}

	for _, sidecar := range deployable.Sidecars {
		if sidecar.Name == "" {
			return fnerrors.InternalError("sidecar name is missing")
		}

		name := sidecarName(sidecar, "sidecar")
		for _, c := range containers {
			if name == c {
				return fnerrors.NewWithLocation(deployable.ErrorLocation, "duplicate sidecar container name: %s", name)
			}
		}
		containers = append(containers, name)

		sidecarSecCtx, err := makeSecurityContext(sidecar.ContainerRunOpts)
		if err != nil {
			return fnerrors.AttachLocation(deployable.ErrorLocation, err)
		}

		scntr := applycorev1.Container().
			WithName(name).
			WithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError).
			WithImage(sidecar.Image.RepoAndDigest()).
			WithArgs(sidecar.Args...).
			WithCommand(sidecar.Command...).
			WithSecurityContext(sidecarSecCtx)

		// XXX remove this
		scntr = scntr.WithEnv(
			applycorev1.EnvVar().WithName("FN_KUBERNETES_NAMESPACE").WithValue(target.namespace),
			applycorev1.EnvVar().WithName("FN_SERVER_ID").WithValue(deployable.GetId()),
			applycorev1.EnvVar().WithName("FN_SERVER_NAME").WithValue(deployable.Name),
		)

		if _, err := fillEnv(ctx, deployable.RuntimeConfig, scntr, sidecar.Env, opts.secrets, seccol, &ensure); err != nil {
			return err
		}

		// Share all mounts with sidecards for now.
		// XXX security review this.
		scntr.VolumeMounts = mainContainer.VolumeMounts
		spec.WithContainers(scntr)
	}

	for _, init := range deployable.Inits {
		if init.Name == "" {
			return fnerrors.InternalError("sidecar name is missing")
		}

		name := sidecarName(init, "init")
		for _, c := range containers {
			if name == c {
				return fnerrors.NewWithLocation(deployable.ErrorLocation, "duplicate init container name: %s", name)
			}
		}

		containers = append(containers, name)

		scntr := applycorev1.Container().
			WithName(name).
			WithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError).
			WithImage(init.Image.RepoAndDigest()).
			WithArgs(append(init.Args, initArgs[init.BinaryRef.Canonical()]...)...).
			WithCommand(init.Command...).
			WithVolumeMounts(initVolumeMounts...)

		if _, err := fillEnv(ctx, deployable.RuntimeConfig, scntr, init.Env, opts.secrets, seccol, &ensure); err != nil {
			return err
		}

		spec.WithInitContainers(scntr)
	}

	s.operations = append(s.operations, seccol.planDeployment(target.namespace, annotations, labels)...)

	podSecCtx := applycorev1.PodSecurityContext()
	if specifiedSec == nil {
		toparse := map[string]any{
			"defaults/pod.podsecuritycontext.yaml": podSecCtx,
		}

		for path, obj := range toparse {
			contents, err := fs.ReadFile(defaults, path)
			if err != nil {
				return fnerrors.InternalError("internal kubernetes data failed to read: %w", err)
			}

			if err := yaml.Unmarshal(contents, obj); err != nil {
				return fnerrors.InternalError("%s: failed to parse defaults: %w", path, err)
			}
		}
	} else {
		if specifiedSec.RunAsUser != 0 {
			podSecCtx = podSecCtx.WithRunAsUser(specifiedSec.RunAsUser)
		}
		if specifiedSec.RunAsGroup != 0 {
			podSecCtx = podSecCtx.WithRunAsGroup(specifiedSec.RunAsGroup)
		}
		if specifiedSec.FsGroup != 0 {
			podSecCtx = podSecCtx.WithFSGroup(specifiedSec.FsGroup)
		}
	}

	if _, err := runAsToPodSecCtx(podSecCtx, deployable.MainContainer.RunAs); err != nil {
		return fnerrors.AttachLocation(deployable.ErrorLocation, err)
	}

	if deployable.MainContainer.Privileged {
		podSecCtx = podSecCtx.WithRunAsUser(0).WithRunAsGroup(0)
	}

	spec = spec.
		WithSecurityContext(podSecCtx).
		WithContainers(mainContainer).
		WithAutomountServiceAccountToken(serviceAccount != "")

	if serviceAccount != "" {
		spec = spec.WithServiceAccountName(serviceAccount)
	}

	tmpl = tmpl.WithSpec(spec)

	if createServiceAccount {
		annotations := map[string]string{}
		for _, ann := range serviceAccountAnnotations {
			annotations[ann.Key] = ann.Value
		}

		s.operations = append(s.operations, kubedef.Apply{
			Description: "Service Account",
			Resource: applycorev1.ServiceAccount(serviceAccount, target.namespace).
				WithLabels(labels).
				WithAnnotations(annotations),
		})
	} else {
		if len(serviceAccountAnnotations) > 0 {
			return fnerrors.NewWithLocation(deployable.ErrorLocation, "can't set service account annotations without ensure_service_account")
		}
	}

	// We don't deploy managed deployments or statefulsets in tests, as these are one-shot
	// servers which we want to control a bit more carefully. For example, we want to deploy
	// them with restart_policy=never, which we would otherwise not be able to do with
	// deployments.
	if deployAsPods(target.env) || isOneShotLike(deployable.Class) {
		desc := fmt.Sprintf("Server %s", deployable.Name)
		if isOneShotLike(deployable.Class) {
			desc = fmt.Sprintf("One-shot %s", deployable.Name)
		}

		ensure.Description = firstStr(deployable.Description, desc)
		ensure.Resource = applycorev1.Pod(deploymentId, target.namespace).
			WithAnnotations(annotations).
			WithAnnotations(tmpl.Annotations).
			WithLabels(labels).
			WithLabels(tmpl.Labels).
			WithSpec(tmpl.Spec.WithRestartPolicy(corev1.RestartPolicyNever))
	} else {
		switch deployable.Class {
		case schema.DeployableClass_STATELESS:
			ensure.Description = firstStr(deployable.Description, fmt.Sprintf("Server Deployment %s", deployable.Name))
			deployment := appsv1.
				Deployment(deploymentId, target.namespace).
				WithAnnotations(annotations).
				WithLabels(labels).
				WithSpec(appsv1.DeploymentSpec().
					WithReplicas(1).
					WithRevisionHistoryLimit(revisionHistoryLimit).
					WithTemplate(tmpl).
					WithSelector(applymetav1.LabelSelector().WithMatchLabels(kubedef.SelectById(deployable))))
			if deployable.ConfigImage != nil {
				deployment.WithAnnotations(map[string]string{
					kubedef.K8sConfigImage: deployable.ConfigImage.RepoAndDigest(),
				})
			}

			ensure.Resource = deployment

		case schema.DeployableClass_STATEFUL:
			ensure.Description = firstStr(deployable.Description, fmt.Sprintf("Server StatefulSet %s", deployable.Name))
			statefulSet := appsv1.
				StatefulSet(deploymentId, target.namespace).
				WithAnnotations(annotations).
				WithLabels(labels).
				WithSpec(appsv1.StatefulSetSpec().
					WithReplicas(1).
					WithRevisionHistoryLimit(revisionHistoryLimit).
					WithTemplate(tmpl).
					WithSelector(applymetav1.LabelSelector().WithMatchLabels(kubedef.SelectById(deployable))))
			if deployable.ConfigImage != nil {
				statefulSet.WithAnnotations(map[string]string{
					kubedef.K8sConfigImage: deployable.ConfigImage.RepoAndDigest(),
				})
			}

			ensure.Resource = statefulSet

		case schema.DeployableClass_DAEMONSET:
			ensure.Description = firstStr(deployable.Description, fmt.Sprintf("Server DaemonSet %s", deployable.Name))
			deployment := appsv1.
				DaemonSet(deploymentId, target.namespace).
				WithAnnotations(annotations).
				WithLabels(labels).
				WithSpec(appsv1.DaemonSetSpec().
					WithRevisionHistoryLimit(revisionHistoryLimit).
					WithTemplate(tmpl).
					WithSelector(applymetav1.LabelSelector().WithMatchLabels(kubedef.SelectById(deployable))))
			if deployable.ConfigImage != nil {
				deployment.WithAnnotations(map[string]string{
					kubedef.K8sConfigImage: deployable.ConfigImage.RepoAndDigest(),
				})
			}

			ensure.Resource = deployment

		default:
			return fnerrors.InternalError("%s: unsupported deployable class", deployable.Class)
		}
	}

	s.operations = append(s.operations, ensure)
	return nil
}

func makeSecurityContext(opts runtime.ContainerRunOpts) (*applycorev1.SecurityContextApplyConfiguration, error) {
	secCtx := applycorev1.SecurityContext()

	const path = "defaults/container.securitycontext.yaml"
	contents, err := fs.ReadFile(defaults, path)
	if err != nil {
		return nil, fnerrors.InternalError("internal kubernetes data failed to read: %w", err)
	}

	if err := yaml.Unmarshal(contents, secCtx); err != nil {
		return nil, fnerrors.InternalError("%s: failed to parse defaults: %w", path, err)
	}

	if opts.Privileged {
		secCtx = secCtx.WithPrivileged(true).WithAllowPrivilegeEscalation(true)
	}

	if opts.ReadOnlyFilesystem {
		secCtx = secCtx.WithReadOnlyRootFilesystem(true)
	}

	return secCtx, nil
}

func firstStr(strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return ""
}

func isOneShotLike(class schema.DeployableClass) bool {
	return class == schema.DeployableClass_ONESHOT || class == schema.DeployableClass_MANUAL
}

type collector struct {
	data       map[string]string
	binaryData map[string][]byte
}

func newDataItemCollector() *collector {
	return &collector{data: map[string]string{}, binaryData: map[string][]byte{}}
}

func (cm *collector) set(key string, rsc *schema.FileContents) {
	if rsc.Utf8 {
		cm.data[key] = string(rsc.Contents)
	} else {
		cm.binaryData[key] = rsc.Contents
	}
}

func makeConfigEntry(hash io.Writer, entry *schema.ConfigurableVolume_Entry, rsc *schema.FileContents, cm *collector) *applycorev1.KeyToPathApplyConfiguration {
	key := naming.DomainFragLike(entry.Path, rsc.Path)
	fmt.Fprintf(hash, "%s:", key)
	_, _ = hash.Write(rsc.Contents)
	fmt.Fprintln(hash)
	cm.set(key, rsc)

	return applycorev1.KeyToPath().WithKey(key)
}

func makePersistentVolume(ns string, env *schema.Environment, loc fnerrors.Location, owner, name, persistentId string, sizeBytes uint64, annotations map[string]string) (*applycorev1.VolumeApplyConfiguration, definitions, error) {
	if sizeBytes >= math.MaxInt64 {
		return nil, nil, fnerrors.NewWithLocation(loc, "requiredstorage value too high (maximum is %d)", math.MaxInt64)
	}

	quantity := resource.NewScaledQuantity(int64(sizeBytes), 0)

	// Ephemeral environments are short lived, so there is no need for persistent volume claims.
	// Admin servers are excluded here as they run as singletons in a global namespace.
	var operations definitions
	var v *applycorev1.VolumeApplyConfiguration

	if env.GetEphemeral() {
		v = applycorev1.Volume().
			WithName(name).
			WithEmptyDir(applycorev1.EmptyDirVolumeSource().
				WithSizeLimit(*quantity))
	} else {
		v = applycorev1.Volume().
			WithName(name).
			WithPersistentVolumeClaim(
				applycorev1.PersistentVolumeClaimVolumeSource().
					WithClaimName(persistentId))

		operations = append(operations, kubedef.Apply{
			Description: fmt.Sprintf("Persistent storage for %s (%s)", owner, humanize.Bytes(sizeBytes)),
			Resource: applycorev1.PersistentVolumeClaim(persistentId, ns).
				WithLabels(kubedef.ManagedByUs()).
				WithAnnotations(annotations).
				WithSpec(applycorev1.PersistentVolumeClaimSpec().
					WithAccessModes(corev1.ReadWriteOnce).
					WithResources(applycorev1.ResourceRequirements().WithRequests(corev1.ResourceList{
						corev1.ResourceStorage: *quantity,
					}))),
		})
	}

	return v, operations, nil
}

func sidecarName(o runtime.SidecarRunOpts, prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, o.Name)
}

func runAsToPodSecCtx(podSecCtx *applycorev1.PodSecurityContextApplyConfiguration, runAs *runtime.RunAs) (*applycorev1.PodSecurityContextApplyConfiguration, error) {
	if runAs != nil {
		if runAs.UserID != "" {
			userId, err := strconv.ParseInt(runAs.UserID, 10, 64)
			if err != nil {
				return nil, fnerrors.InternalError("expected server.RunAs.UserID to be an int64: %w", err)
			}

			if podSecCtx.RunAsUser != nil && *podSecCtx.RunAsUser != userId {
				return nil, fnerrors.BadInputError("incompatible userid %d vs %d (in RunAs)", *podSecCtx.RunAsUser, userId)
			}

			podSecCtx = podSecCtx.WithRunAsUser(userId).WithRunAsNonRoot(true)
		}

		if runAs.FSGroup != nil {
			fsGroup, err := strconv.ParseInt(*runAs.FSGroup, 10, 64)
			if err != nil {
				return nil, fnerrors.InternalError("expected server.RunAs.FSGroup to be an int64: %w", err)
			}

			if podSecCtx.FSGroup != nil && *podSecCtx.FSGroup != fsGroup {
				return nil, fnerrors.BadInputError("incompatible fsgroup %d vs %d (in RunAs)", *podSecCtx.FSGroup, fsGroup)
			}

			podSecCtx.WithFSGroup(fsGroup)
		}

		return podSecCtx, nil
	}

	return nil, nil
}

func deployEndpoint(ctx context.Context, r clusterTarget, deployable runtime.DeployableSpec, endpoint *schema.Endpoint, s *serverRunState) error {
	serviceSpec := applycorev1.ServiceSpec().WithSelector(kubedef.SelectById(deployable))

	port := endpoint.Port
	if port != nil {
		serviceSpec = serviceSpec.WithPorts(applycorev1.ServicePort().
			WithProtocol(corev1.ProtocolTCP).WithName(port.Name).WithPort(port.ContainerPort))

		if endpoint.Type == schema.Endpoint_LOAD_BALANCER {
			serviceSpec = serviceSpec.WithType(corev1.ServiceTypeLoadBalancer)
		}

		serviceAnnotations, err := kubedef.MakeServiceAnnotations(endpoint)
		if err != nil {
			return err
		}

		for _, md := range endpoint.ServiceMetadata {
			x := &schema.ServiceAnnotations{}
			if md.Details.MessageIs(x) {
				if err := md.Details.UnmarshalTo(x); err != nil {
					return fnerrors.InternalError("failed to unmarshal ServiceAnnotations: %w", err)
				}
				for _, kv := range x.KeyValue {
					serviceAnnotations[kv.Key] = kv.Value
				}
			}
		}

		if err := validateServiceName(endpoint.AllocatedName); err != nil {
			return fnerrors.AttachLocation(deployable.ErrorLocation, err)
		}

		s.operations = append(s.operations, kubedef.Apply{
			Description: fmt.Sprintf("Service %s:%s", deployable.Name, endpoint.ServiceName),
			Resource: applycorev1.
				Service(endpoint.AllocatedName, r.namespace).
				WithLabels(kubedef.MakeServiceLabels(r.env, deployable, endpoint)).
				WithAnnotations(serviceAnnotations).
				WithSpec(serviceSpec),
			SchedCategory: []string{
				runtime.OwnedByDeployable(deployable),
				kubedef.MakeServicesCat(deployable),
			},
			SchedAfterCategory: []string{
				runtime.DeployableCategory(deployable),
			},
		})
	}

	return nil
}

func toK8sVol(vol *kubedef.SpecExtension_Volume) (*applycorev1.VolumeApplyConfiguration, error) {
	v := applycorev1.Volume().WithName(vol.Name)
	switch x := vol.VolumeType.(type) {
	case *kubedef.SpecExtension_Volume_Secret_:
		return v.WithSecret(applycorev1.SecretVolumeSource().WithSecretName(x.Secret.SecretName)), nil
	case *kubedef.SpecExtension_Volume_ConfigMap_:
		vol := applycorev1.ConfigMapVolumeSource().WithName(x.ConfigMap.Name)
		for _, it := range x.ConfigMap.Item {
			vol = vol.WithItems(applycorev1.KeyToPath().WithKey(it.Key).WithPath(it.Path))
		}
		return v.WithConfigMap(vol), nil
	default:
		return nil, fnerrors.InternalError("don't know how to instantiate a k8s volume from %v", vol)
	}
}

func generatedSecretName(spec *schema.SecretSpec_GenerateSpec) (string, string) {
	return fmt.Sprintf("gen-%s", spec.UniqueId), "generated"
}
