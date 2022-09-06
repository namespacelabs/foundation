// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/storage"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/go-ids"
	"sigs.k8s.io/yaml"
)

var DeployAsPodsInTests = true

const (
	kubeNode schema.PackageName = "namespacelabs.dev/foundation/std/runtime/kubernetes"

	runtimeConfigVersion = 0
)

type perEnvConf struct {
	dashnessPeriod        int32
	livenessInitialDelay  int32
	readinessInitialDelay int32
	probeTimeout          int32
	failureThreshold      int32
}

var constants = map[schema.Environment_Purpose]perEnvConf{
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

func lookupByName(env []*schema.BinaryConfig_EnvEntry, name string) (*schema.BinaryConfig_EnvEntry, bool) {
	for _, env := range env {
		if env.Name == name {
			return env, true
		}
	}

	return nil, false
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

func toProbe(port *schema.Endpoint_Port, md *schema.ServiceMetadata) (*kubedef.ContainerExtension_Probe, error) {
	exported := &schema.HttpExportedService{}
	if err := md.Details.UnmarshalTo(exported); err != nil {
		return nil, fnerrors.InternalError("expected a HttpExportedService: %w", err)
	}

	return &kubedef.ContainerExtension_Probe{Path: exported.GetPath(), ContainerPort: port.GetContainerPort(), Kind: md.Kind}, nil
}

func toK8sProbe(p *applycorev1.ProbeApplyConfiguration, probevalues perEnvConf, probe *kubedef.ContainerExtension_Probe) *applycorev1.ProbeApplyConfiguration {
	return p.WithHTTPGet(applycorev1.HTTPGetAction().WithPath(probe.GetPath()).
		WithPort(intstr.FromInt(int(probe.GetContainerPort())))).
		WithPeriodSeconds(probevalues.dashnessPeriod).
		WithFailureThreshold(probevalues.failureThreshold).
		WithTimeoutSeconds(probevalues.probeTimeout)
}

type deployOpts struct {
	focus    schema.PackageList
	stackIds []string
	secrets  runtime.GroundedSecrets
}

func deployAsPods(env *schema.Environment) bool {
	return env.Purpose == schema.Environment_TESTING && DeployAsPodsInTests
}

func (r K8sRuntime) prepareServerDeployment(ctx context.Context, server runtime.ServerConfig, internalEndpoints []*schema.InternalEndpoint, opts deployOpts, s *serverRunState) error {
	srv := server.Server
	ns := serverNamespace(r, srv.Proto())

	if server.Image.Repository == "" {
		return fnerrors.InternalError("kubernetes: no repository defined in image: %v", server.Image)
	}

	probevalues, ok := constants[r.env.Purpose]
	if !ok {
		return fnerrors.InternalError("%s: no constants configured", r.env.Name)
	}

	kubepkg, err := srv.SealedContext().LoadByName(ctx, kubeNode)
	if err != nil {
		return err
	}

	secCtx := applycorev1.SecurityContext()

	if server.ReadOnlyFilesystem {
		secCtx = secCtx.WithReadOnlyRootFilesystem(true)
	}

	name := kubedef.ServerCtrName(srv.Proto())
	containers := []string{name}
	container := applycorev1.Container().
		WithName(name).
		WithImage(server.Image.RepoAndDigest()).
		WithArgs(server.Args...).
		WithCommand(server.Command...).
		WithSecurityContext(secCtx)

	var probes []*kubedef.ContainerExtension_Probe
	for _, internal := range internalEndpoints {
		for _, md := range internal.ServiceMetadata {
			if md.Kind == runtime.FnServiceLivez || md.Kind == runtime.FnServiceReadyz {
				probe, err := toProbe(internal.GetPort(), md)
				if err != nil {
					return err
				}

				probes = append(probes, probe)
			}
		}
	}

	if _, err := fillEnv(container, server.Env); err != nil {
		return err
	}

	if server.WorkingDir != "" {
		container = container.WithWorkingDir(server.WorkingDir)
	}

	spec := applycorev1.PodSpec().
		WithEnableServiceLinks(false) // Disable service injection via environment variables.

	var labels map[string]string
	if srv.Proto().ClusterAdmin {
		// Admin servers are environment agnostic (deployed in a single global namespace).
		labels = kubedef.MakeLabels(nil, srv.Proto())
	} else {
		labels = kubedef.MakeLabels(r.env, srv.Proto())
	}

	annotations := kubedef.MakeAnnotations(r.env, srv.StackEntry())

	if opts.focus.Includes(srv.PackageName()) {
		labels = kubedef.WithFocusMark(labels)
	}

	deploymentId := kubedef.MakeDeploymentId(srv.Proto())

	tmpl := applycorev1.PodTemplateSpec().
		WithAnnotations(annotations).
		WithLabels(labels)

	var initVolumeMounts []*applycorev1.VolumeMountApplyConfiguration
	// Key: PackageRef.CanonicalString().
	initArgs := map[string][]string{}

	var serviceAccount string // May be specified by a SpecExtension.
	var createServiceAccount bool
	var serviceAccountAnnotations []*kubedef.SpecExtension_Annotation

	var env []*schema.BinaryConfig_EnvEntry
	var specifiedSec *kubedef.SpecExtension_SecurityContext

	for _, input := range server.Extensions {
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
					return fnerrors.UserError(server.Server.Location, "ensure_service_account requires service_account to be set")
				}
			}

			serviceAccountAnnotations = append(serviceAccountAnnotations, specExt.ServiceAccountAnnotation...)

			if specExt.ServiceAccount != "" {
				if serviceAccount != "" && serviceAccount != specExt.ServiceAccount {
					return fnerrors.UserError(server.Server.Location, "incompatible service accounts defined, %q vs %q",
						serviceAccount, specExt.ServiceAccount)
				}
				serviceAccount = specExt.ServiceAccount
			}

			if specExt.SecurityContext != nil {
				if specifiedSec == nil {
					specifiedSec = specExt.SecurityContext
				} else if !proto.Equal(specifiedSec, specExt.SecurityContext) {
					return fnerrors.UserError(server.Server.Package.Server, "incompatible securitycontext defined, %v vs %v",
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
				container = container.WithVolumeMounts(volumeMount)
				if mount.MountOnInit {
					// Volume mounts may declare to be available also during server initialization.
					// E.g. Initializing the schema of a data store requires early access to server secrets.
					// The volume mount provider has full control over whether the volume is available.
					initVolumeMounts = append(initVolumeMounts, volumeMount)
				}
			}

			// XXX O(n^2)
			for _, kv := range containerExt.Env {
				if current, found := lookupByName(env, kv.Name); found && !proto.Equal(current, kv) {
					return fnerrors.UserError(server.Server.Location, "env variable %q is already set, but would be overwritten by container extension", kv.Name)
				}

				env = append(env, kv)
			}

			if containerExt.Args != nil {
				container = container.WithArgs(containerExt.Args...)
			} else {
				// Deprecated path.
				for _, arg := range containerExt.ArgTuple {
					if currentValue, found := getArg(container, arg.Name); found && currentValue != arg.Value {
						return fnerrors.UserError(server.Server.Location, "argument '%s' is already set to '%s' but would be overwritten to '%s' by container extension", arg.Name, currentValue, arg.Value)
					}
					container = container.WithArgs(fmt.Sprintf("--%s=%s", arg.Name, arg.Value))
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

	var readinessProbe, livenessProbe *kubedef.ContainerExtension_Probe
	for _, probe := range probes {
		switch probe.Kind {
		case runtime.FnServiceLivez:
			if livenessProbe == nil {
				livenessProbe = probe
			} else if !proto.Equal(probe, livenessProbe) {
				return fnerrors.BadInputError("inconsistent live probe definition")
			}
		case runtime.FnServiceReadyz:
			if readinessProbe == nil {
				readinessProbe = probe
			} else if !proto.Equal(probe, readinessProbe) {
				return fnerrors.BadInputError("inconsistent ready probe definition")
			}
		default:
			return fnerrors.BadInputError("%s: unknown probe kind", probe.Kind)
		}
	}

	if readinessProbe != nil {
		container = container.WithReadinessProbe(
			toK8sProbe(applycorev1.Probe().WithInitialDelaySeconds(probevalues.readinessInitialDelay),
				probevalues, readinessProbe))
	}

	if livenessProbe != nil {
		container = container.WithLivenessProbe(
			toK8sProbe(applycorev1.Probe().WithInitialDelaySeconds(probevalues.livenessInitialDelay),
				probevalues, livenessProbe))
	}

	if _, err := fillEnv(container, env); err != nil {
		return err
	}

	volumes := slices.Clone(srv.Proto().Volumes)
	mounts := slices.Clone(srv.Proto().Mounts)

	for _, ext := range server.ServerExtensions {
		volumes = append(volumes, ext.Volume...)
		mounts = append(mounts, ext.Mount...)
	}

	for k, volume := range volumes {
		if volume.Name == "" {
			return fnerrors.InternalError("volume #%d is missing a name", k)
		}

		name := fmt.Sprintf("v-%s", volume.Name)

		switch volume.Kind {
		case storage.VolumeKindEphemeral:
			spec = spec.WithVolumes(applycorev1.Volume().WithName(name).WithEmptyDir(applycorev1.EmptyDirVolumeSource()))

		case storage.VolumeKindPersistent:
			pv := &schema.PersistentVolume{}
			if err := volume.Definition.UnmarshalTo(pv); err != nil {
				return fnerrors.InternalError("%s: failed to unmarshal persistent volume definition: %w", volume.Name, err)
			}

			if pv.Id == "" {
				return fnerrors.BadInputError("%s: persistent ID is missing", volume.Name)
			}

			v, operations, err := makePersistentVolume(ns, r.env, srv, volume.Owner, name, pv.Id, pv.SizeBytes)
			if err != nil {
				return err
			}

			spec = spec.WithVolumes(v)
			s.operations = append(s.operations, operations...)

		case storage.VolumeKindConfigurable:
			cv := &schema.ConfigurableVolume{}
			if err := volume.Definition.UnmarshalTo(cv); err != nil {
				return fnerrors.InternalError("%s: failed to unmarshal configurable volume definition: %w", volume.Name, err)
			}

			configs := newDataItemCollector()
			secrets := newDataItemCollector()

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
					resource := opts.secrets.Get(entry.SecretRef.Owner, entry.SecretRef.Name)
					if resource == nil {
						return fnerrors.BadInputError("%s/%s: missing secret value", entry.SecretRef.Owner, entry.SecretRef.Name)
					}

					secretItems = append(secretItems, makeConfigEntry(configHash, entry, resource, secrets).WithPath(entry.Path))

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
					Resource: applycorev1.ConfigMap(configId, ns).
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
				// XXX Think through this, should it also be hashed + immutable?
				secretId := fmt.Sprintf("ns-managed-%s-%s", srv.Name(), srv.Proto().Id)

				projected = projected.WithSources(applycorev1.VolumeProjection().WithSecret(
					applycorev1.SecretProjection().WithName(secretId).WithItems(secretItems...)))

				// Needs to be declared before it's used.
				s.operations = append(s.operations, kubedef.Apply{
					Description: "Server secrets",
					Resource: applycorev1.Secret(secretId, ns).
						WithAnnotations(annotations).
						WithLabels(labels).
						WithLabels(map[string]string{
							kubedef.K8sKind: kubedef.K8sStaticConfigKind,
						}).
						WithStringData(configs.data).
						WithData(configs.binaryData),
				})
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

		if mount.VolumeName == "" {
			return fnerrors.InternalError("mount %q is missing a target volume", mount.Path)
		}

		volumeName := fmt.Sprintf("v-%s", mount.VolumeName)
		container = container.WithVolumeMounts(applycorev1.VolumeMount().
			WithMountPath(mount.Path).
			WithName(volumeName).
			WithReadOnly(mount.Readonly))
	}

	// Before sidecars so they have access to the "runtime config" volume.
	if server.RuntimeConfig != nil {
		serializedConfig, err := json.Marshal(server.RuntimeConfig)
		if err != nil {
			return fnerrors.InternalError("failed to serialize runtime configuration: %w", err)
		}

		configDigest, err := schema.DigestOf(map[string]any{
			"version": runtimeConfigVersion,
			"config":  serializedConfig,
		})
		if err != nil {
			return fnerrors.InternalError("failed to digest runtime configuration: %w", err)
		}

		configId := deploymentId + "-rtconfig-" + configDigest.Hex[:8]

		// Needs to be declared before it's used.
		s.operations = append(s.operations, kubedef.Apply{
			Description: "Runtime configuration",
			Resource: applycorev1.ConfigMap(configId, ns).
				WithAnnotations(annotations).
				WithLabels(labels).
				WithLabels(map[string]string{
					kubedef.K8sKind: kubedef.K8sRuntimeConfigKind,
				}).
				WithImmutable(true).
				WithData(map[string]string{
					"runtime.json": string(serializedConfig),
				}),
		})

		spec = spec.WithVolumes(applycorev1.Volume().
			WithName(configId).
			WithConfigMap(applycorev1.ConfigMapVolumeSource().WithName(configId)))

		container = container.WithVolumeMounts(applycorev1.VolumeMount().WithMountPath("/namespace/config").WithName(configId).WithReadOnly(true))

		// We do manual cleanup of unused configs. In the future they'll be owned by a deployment intent.
		annotations[kubedef.K8sRuntimeConfig] = configId
	}

	for _, sidecar := range server.Sidecars {
		if sidecar.Name == "" {
			return fnerrors.InternalError("sidecar name is missing")
		}

		name := sidecarName(sidecar, "sidecar")
		for _, c := range containers {
			if name == c {
				return fnerrors.UserError(server.Server.Location, "duplicate sidecar container name: %s", name)
			}
		}
		containers = append(containers, name)

		scntr := applycorev1.Container().
			WithName(name).
			WithImage(sidecar.Image.RepoAndDigest()).
			WithArgs(sidecar.Args...).
			WithCommand(sidecar.Command...)

		// XXX remove this
		scntr = scntr.WithEnv(
			applycorev1.EnvVar().WithName("FN_KUBERNETES_NAMESPACE").WithValue(ns),
			applycorev1.EnvVar().WithName("FN_SERVER_ID").WithValue(srv.Proto().Id),
			applycorev1.EnvVar().WithName("FN_SERVER_NAME").WithValue(srv.Proto().Name),
		)

		if _, err := fillEnv(scntr, sidecar.Env); err != nil {
			return err
		}

		// Share all mounts with sidecards for now.
		// XXX security review this.
		scntr.VolumeMounts = container.VolumeMounts
		spec.WithContainers(scntr)
	}

	for _, init := range server.Inits {
		if init.Name == "" {
			return fnerrors.InternalError("sidecar name is missing")
		}

		name := sidecarName(init, "init")
		for _, c := range containers {
			if name == c {
				return fnerrors.UserError(server.Server.Location, "duplicate init container name: %s", name)
			}
		}
		containers = append(containers, name)

		spec.WithInitContainers(
			applycorev1.Container().
				WithName(name).
				WithImage(init.Image.RepoAndDigest()).
				WithArgs(append(init.Args, initArgs[init.PackageRef.Canonical()]...)...).
				WithCommand(init.Command...).
				WithVolumeMounts(initVolumeMounts...))
	}

	podSecCtx := applycorev1.PodSecurityContext()
	if specifiedSec == nil {
		toparse := map[string]interface{}{
			"defaults/container.securitycontext.yaml": secCtx,
			"defaults/pod.podsecuritycontext.yaml":    podSecCtx,
		}

		for _, data := range kubepkg.PackageData {
			if obj, ok := toparse[data.Path]; ok {
				if err := yaml.Unmarshal(data.Contents, obj); err != nil {
					return fnerrors.InternalError("%s: failed to parse defaults: %w", data.Path, err)
				}
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

	if _, err := runAsToPodSecCtx(server.Server.PackageName().String(), podSecCtx, server.RunAs); err != nil {
		return err
	}

	spec = spec.
		WithSecurityContext(podSecCtx).
		WithContainers(container).
		WithAutomountServiceAccountToken(serviceAccount != "")

	if serviceAccount != "" {
		spec = spec.WithServiceAccountName(serviceAccount)
	}

	tmpl = tmpl.WithSpec(spec)

	// Only mutate `annotations` after all other uses above.
	if server.ConfigImage != nil {
		annotations[kubedef.K8sConfigImage] = server.ConfigImage.RepoAndDigest()
	}

	if createServiceAccount {
		annotations := map[string]string{}
		for _, ann := range serviceAccountAnnotations {
			annotations[ann.Key] = ann.Value
		}

		s.operations = append(s.operations, kubedef.Apply{
			Description: "Service Account",
			Resource: applycorev1.ServiceAccount(serviceAccount, ns).
				WithLabels(labels).
				WithAnnotations(annotations),
		})
	} else {
		if len(serviceAccountAnnotations) > 0 {
			return fnerrors.UserError(server.Server.Location, "can't set service account annotations without ensure_service_account")
		}
	}

	// We don't deploy managed deployments or statefulsets in tests, as these are one-shot
	// servers which we want to control a bit more carefully. For example, we want to deploy
	// them with restart_policy=never, which we would otherwise not be able to do with
	// deployments.
	// Admin servers are excluded here as they run as singletons in a global namespace.
	if deployAsPods(r.env) && !srv.Proto().ClusterAdmin {
		s.operations = append(s.operations, kubedef.Apply{
			Description: "Server",
			Resource: applycorev1.Pod(deploymentId, ns).
				WithAnnotations(annotations).
				WithAnnotations(tmpl.Annotations).
				WithLabels(labels).
				WithLabels(tmpl.Labels).
				WithSpec(tmpl.Spec.WithRestartPolicy(corev1.RestartPolicyNever)),
		})
		return nil
	}

	if server.Server.IsStateful() {
		s.operations = append(s.operations, kubedef.Apply{
			Description: "Server StatefulSet",
			Resource: appsv1.
				StatefulSet(deploymentId, ns).
				WithAnnotations(annotations).
				WithLabels(labels).
				WithSpec(appsv1.StatefulSetSpec().
					WithReplicas(1).
					WithTemplate(tmpl).
					WithSelector(applymetav1.LabelSelector().WithMatchLabels(kubedef.SelectById(srv.Proto())))),
		})
	} else {
		s.operations = append(s.operations, kubedef.Apply{
			Description: "Server Deployment",
			Resource: appsv1.
				Deployment(deploymentId, ns).
				WithAnnotations(annotations).
				WithLabels(labels).
				WithSpec(appsv1.DeploymentSpec().
					WithReplicas(1).
					WithTemplate(tmpl).
					WithSelector(applymetav1.LabelSelector().WithMatchLabels(kubedef.SelectById(srv.Proto())))),
		})
	}

	return nil
}

type collector struct {
	data       map[string]string
	binaryData map[string][]byte
}

func newDataItemCollector() *collector {
	return &collector{data: map[string]string{}, binaryData: map[string][]byte{}}
}

func (cm *collector) set(key string, rsc *schema.Resource) {
	if rsc.Utf8 {
		cm.data[key] = string(rsc.Contents)
	} else {
		cm.binaryData[key] = rsc.Contents
	}
}

func makeConfigEntry(h io.Writer, entry *schema.ConfigurableVolume_Entry, rsc *schema.Resource, cm *collector) *applycorev1.KeyToPathApplyConfiguration {
	key := fmt.Sprintf("%s.%s", ids.EncodeToBase62String([]byte(entry.Path)), ids.EncodeToBase62String([]byte(rsc.Path)))

	fmt.Fprintf(h, "%s:", key)
	_, _ = h.Write(rsc.Contents)
	fmt.Fprintln(h)
	cm.set(key, rsc)

	return applycorev1.KeyToPath().WithKey(key)
}

func makePersistentVolume(ns string, env *schema.Environment, srv provision.Server, owner, name, persistentId string, sizeBytes uint64) (*applycorev1.VolumeApplyConfiguration, []kubedef.Apply, error) {
	if sizeBytes >= math.MaxInt64 {
		return nil, nil, fnerrors.UserError(srv.Location, "requiredstorage value too high (maximum is %d)", math.MaxInt64)
	}

	quantity := resource.NewScaledQuantity(int64(sizeBytes), resource.Scale(1))

	// Ephemeral environments are short lived, so there is no need for persistent volume claims.
	// Admin servers are excluded here as they run as singletons in a global namespace.
	var operations []kubedef.Apply
	var v *applycorev1.VolumeApplyConfiguration

	if env.Ephemeral && !srv.Proto().ClusterAdmin {
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

func runAsToPodSecCtx(name string, podSecCtx *applycorev1.PodSecurityContextApplyConfiguration, runAs *runtime.RunAs) (*applycorev1.PodSecurityContextApplyConfiguration, error) {
	if runAs != nil {
		if runAs.UserID != "" {
			userId, err := strconv.ParseInt(runAs.UserID, 10, 64)
			if err != nil {
				return nil, fnerrors.InternalError("expected server.RunAs.UserID to be an int64: %w", err)
			}

			if podSecCtx.RunAsUser != nil && *podSecCtx.RunAsUser != userId {
				return nil, fnerrors.BadInputError("%s: incompatible userid %d vs %d (in RunAs)", name, *podSecCtx.RunAsUser, userId)
			}

			podSecCtx = podSecCtx.WithRunAsUser(userId).WithRunAsNonRoot(true)
		}

		if runAs.FSGroup != nil {
			fsGroup, err := strconv.ParseInt(*runAs.FSGroup, 10, 64)
			if err != nil {
				return nil, fnerrors.InternalError("expected server.RunAs.FSGroup to be an int64: %w", err)
			}

			if podSecCtx.FSGroup != nil && *podSecCtx.FSGroup != fsGroup {
				return nil, fnerrors.BadInputError("%s: incompatible fsgroup %d vs %d (in RunAs)", name, *podSecCtx.FSGroup, fsGroup)
			}

			podSecCtx.WithFSGroup(fsGroup)
		}

		return podSecCtx, nil
	}

	return nil, nil
}

func fillEnv(container *applycorev1.ContainerApplyConfiguration, env []*schema.BinaryConfig_EnvEntry) (*applycorev1.ContainerApplyConfiguration, error) {
	for _, kv := range env {
		entry := applycorev1.EnvVar().WithName(kv.Name)
		if kv.ExperimentalFromSecret != "" {
			parts := strings.SplitN(kv.ExperimentalFromSecret, ":", 2)
			if len(parts) < 2 {
				return nil, fnerrors.New("invalid experimental_from_secret format")
			}
			entry = entry.WithValueFrom(applycorev1.EnvVarSource().WithSecretKeyRef(
				applycorev1.SecretKeySelector().WithName(parts[0]).WithKey(parts[1])))
		} else {
			entry = entry.WithValue(kv.Value)
		}
		container = container.WithEnv(entry)
	}

	return container, nil
}

func (r K8sRuntime) deployEndpoint(ctx context.Context, server runtime.ServerConfig, endpoint *schema.Endpoint, s *serverRunState) error {
	t := server.Server
	ns := serverNamespace(r, t.Proto())

	serviceSpec := applycorev1.ServiceSpec().WithSelector(kubedef.SelectById(t.Proto()))

	port := endpoint.Port
	if port != nil {
		serviceSpec = serviceSpec.WithPorts(applycorev1.ServicePort().
			WithProtocol(corev1.ProtocolTCP).WithName(port.Name).WithPort(port.ContainerPort))

		serviceAnnotations, err := kubedef.MakeServiceAnnotations(t.Proto(), endpoint)
		if err != nil {
			return err
		}

		s.operations = append(s.operations, kubedef.Apply{
			Description: fmt.Sprintf("Service %s", endpoint.ServiceName),
			Resource: applycorev1.
				Service(endpoint.AllocatedName, ns).
				WithLabels(kubedef.MakeServiceLabels(r.env, t.Proto(), endpoint)).
				WithAnnotations(serviceAnnotations).
				WithSpec(serviceSpec),
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
