// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/controller"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"sigs.k8s.io/yaml"
)

const kubeNode schema.PackageName = "namespacelabs.dev/foundation/std/runtime/kubernetes"

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

func toProbe(endpoint *schema.InternalEndpoint, md *schema.ServiceMetadata) (*applycorev1.ProbeApplyConfiguration, error) {
	exported := &schema.HttpExportedService{}
	if err := md.Details.UnmarshalTo(exported); err != nil {
		return nil, fnerrors.InternalError("expected a HttpExportedService: %w", err)
	}

	return applycorev1.Probe().WithHTTPGet(
		applycorev1.HTTPGetAction().WithPath(exported.GetPath()).
			WithPort(intstr.FromInt(int(endpoint.GetPort().GetContainerPort())))), nil
}

type deployOpts struct {
	focus    schema.PackageList
	stackIds []string
}

func (r boundEnv) prepareServerDeployment(ctx context.Context, server runtime.ServerConfig, internalEndpoints []*schema.InternalEndpoint, opts deployOpts, s *serverRunState) error {
	srv := server.Server
	isController := controller.IsController(srv.PackageName())
	ns := serverNamespace(r, srv.Proto())

	if server.Image.Repository == "" {
		return fnerrors.InternalError("kubernetes: no repository defined in image: %v", server.Image)
	}

	c, ok := constants[r.host.Env.Purpose]
	if !ok {
		return fnerrors.InternalError("%s: no constants configured", r.host.Env.Name)
	}

	kubepkg, err := srv.Env().LoadByName(ctx, kubeNode)
	if err != nil {
		return err
	}

	secCtx := applycorev1.SecurityContext()
	podSecCtx := applycorev1.PodSecurityContext()

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

	if server.ReadOnlyFilesystem {
		secCtx = secCtx.WithReadOnlyRootFilesystem(true)
	}

	if _, err := runAsToPodSecCtx(podSecCtx, server.RunAs); err != nil {
		return err
	}

	name := serverCtrName(srv.Proto())
	containers := []string{name}
	container := applycorev1.Container().
		WithName(name).
		WithImage(server.Image.RepoAndDigest()).
		WithArgs(server.Args...).
		WithCommand(server.Command...).
		WithSecurityContext(secCtx)

	for _, internal := range internalEndpoints {
		for _, md := range internal.ServiceMetadata {
			if md.Kind == runtime.FnServiceLivez || md.Kind == runtime.FnServiceReadyz {
				probe, err := toProbe(internal, md)
				if err != nil {
					return err
				}

				probe = probe.WithPeriodSeconds(c.dashnessPeriod).WithFailureThreshold(c.failureThreshold).WithTimeoutSeconds(c.probeTimeout)

				switch md.Kind {
				case runtime.FnServiceLivez:
					container = container.WithLivenessProbe(probe.WithInitialDelaySeconds(c.livenessInitialDelay))
				case runtime.FnServiceReadyz:
					container = container.WithReadinessProbe(probe.WithInitialDelaySeconds(c.readinessInitialDelay))
				}
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
		WithSecurityContext(podSecCtx)

	var labels map[string]string
	if isController {
		// Controllers are environment agnostic (deployed in a single global namespace).
		labels = kubedef.MakeLabels(nil, srv.Proto())
	} else {
		labels = kubedef.MakeLabels(r.host.Env, srv.Proto())
	}

	annotations := kubedef.MakeAnnotations(r.host.Env, srv.StackEntry())

	if opts.focus.Includes(srv.PackageName()) {
		labels = kubedef.WithFocusMark(labels)
		annotations = kubedef.WithFocusStack(annotations, opts.stackIds)
	}

	deploymentId := kubedef.MakeDeploymentId(srv.Proto())

	tmpl := applycorev1.PodTemplateSpec().
		WithAnnotations(annotations).
		WithLabels(labels)

	var initVolumeMounts []*applycorev1.VolumeMountApplyConfiguration
	initArgs := map[schema.PackageName][]string{}

	var serviceAccount string // May be specified by a SpecExtension.
	var createServiceAccount bool
	var serviceAccountAnnotations []*kubedef.SpecExtension_Annotation

	var env []*schema.BinaryConfig_EnvEntry

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

			// Deprecated path.
			for _, initContainer := range containerExt.InitContainer {
				pkg := schema.PackageName(initContainer.PackageName)
				initArgs[pkg] = append(initArgs[pkg], initContainer.Arg...)
			}

		case input.Impl.MessageIs(initContainerExt):
			if err := input.Impl.UnmarshalTo(initContainerExt); err != nil {
				return fnerrors.InternalError("failed to unmarshal InitContainerExtension: %w", err)
			}

			pkg := schema.PackageName(initContainerExt.PackageName)
			initArgs[pkg] = append(initArgs[pkg], initContainerExt.Args...)

		default:
			return fnerrors.InternalError("unused startup input: %s", input.Impl.GetTypeUrl())
		}
	}

	if _, err := fillEnv(container, env); err != nil {
		return err
	}

	for _, rs := range srv.Proto().RequiredStorage {
		if rs.Owner == "" {
			return fnerrors.UserError(server.Server.Location, "requiredstorage owner is not set")
		}

		if rs.ByteCount >= math.MaxInt64 {
			return fnerrors.UserError(server.Server.Location, "requiredstorage value too high (maximum is %d)", math.MaxInt64)
		}

		container = container.WithVolumeMounts(
			applycorev1.VolumeMount().
				WithName(makeStorageVolumeName(rs)).
				WithMountPath(rs.MountPath))
		spec = spec.WithVolumes(applycorev1.Volume().
			WithName(makeStorageVolumeName(rs)).
			WithPersistentVolumeClaim(
				applycorev1.PersistentVolumeClaimVolumeSource().
					WithClaimName(rs.PersistentId)))

		s.declarations = append(s.declarations, kubedef.Apply{
			Description: fmt.Sprintf("Persistent storage for %s", rs.Owner),
			Resource:    "persistentvolumeclaims",
			Namespace:   ns,
			Name:        rs.PersistentId,
			Body: applycorev1.PersistentVolumeClaim(rs.PersistentId, ns).
				WithSpec(applycorev1.PersistentVolumeClaimSpec().
					WithAccessModes(corev1.ReadWriteOnce).
					WithResources(applycorev1.ResourceRequirements().WithRequests(corev1.ResourceList{
						corev1.ResourceStorage: *resource.NewScaledQuantity(int64(rs.ByteCount), resource.Scale(1)),
					}))),
		})
	}

	for _, sidecar := range server.Sidecars {
		name := fmt.Sprintf("sidecar-%s", shortPackageName(sidecar.PackageName))
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

		// Share all mounts with sidecards for now.
		// XXX security review this.
		scntr.VolumeMounts = container.VolumeMounts
		spec.WithContainers(scntr)
	}

	for _, init := range server.Inits {
		name := fmt.Sprintf("init-%v", labelName(init.Command))
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
				WithArgs(append(init.Args, initArgs[init.PackageName]...)...).
				WithCommand(init.Command...).
				WithVolumeMounts(initVolumeMounts...))
	}

	spec = spec.
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

		s.declarations = append(s.declarations, kubedef.Apply{
			Description: "Service Account",
			Resource:    "serviceaccounts",
			Namespace:   ns,
			Name:        serviceAccount,
			Body: applycorev1.ServiceAccount(serviceAccount, ns).
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
	// Controllers are excluded here as they run as singletons in a global namespace.
	if r.host.Env.Purpose == schema.Environment_TESTING && !controller.IsController(srv.PackageName()) {
		s.declarations = append(s.declarations, kubedef.Apply{
			Description: "Server",
			Resource:    "pods",
			Namespace:   ns,
			Name:        deploymentId,
			Body: applycorev1.Pod(deploymentId, ns).
				WithAnnotations(annotations).
				WithAnnotations(tmpl.Annotations).
				WithLabels(labels).
				WithLabels(tmpl.Labels).
				WithSpec(tmpl.Spec.WithRestartPolicy(corev1.RestartPolicyNever)),
		})
		return nil
	}

	if server.Server.IsStateful() {
		s.declarations = append(s.declarations, kubedef.Apply{
			Description: "Server StatefulSet",
			Resource:    "statefulsets",
			Namespace:   ns,
			Name:        deploymentId,
			Body: appsv1.
				StatefulSet(deploymentId, ns).
				WithAnnotations(annotations).
				WithLabels(labels).
				WithSpec(appsv1.StatefulSetSpec().
					WithReplicas(1).
					WithTemplate(tmpl).
					WithSelector(applymetav1.LabelSelector().WithMatchLabels(kubedef.SelectById(srv.Proto())))),
		})
	} else {
		s.declarations = append(s.declarations, kubedef.Apply{
			Description: "Server Deployment",
			Resource:    "deployments",
			Namespace:   ns,
			Name:        deploymentId,
			Body: appsv1.
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

func serverCtrName(server *schema.Server) string {
	return strings.ToLower(server.Name) // k8s doesn't accept uppercase names.
}

func makeStorageVolumeName(rs *schema.RequiredStorage) string {
	h := sha256.New()
	fmt.Fprint(h, rs.Owner)
	return "rs-" + hex.EncodeToString(h.Sum(nil))[:8]
}

func runAsToPodSecCtx(podSecCtx *applycorev1.PodSecurityContextApplyConfiguration, runAs *runtime.RunAs) (*applycorev1.PodSecurityContextApplyConfiguration, error) {
	if runAs != nil {
		userId, err := strconv.ParseInt(runAs.UserID, 10, 64)
		if err != nil {
			return nil, fnerrors.InternalError("expected server.RunAs.UserID to be an int64: %w", err)
		}

		podSecCtx = podSecCtx.WithRunAsUser(userId).WithRunAsNonRoot(true)

		if runAs.FSGroup != nil {
			fsGroup, err := strconv.ParseInt(*runAs.FSGroup, 10, 64)
			if err != nil {
				return nil, fnerrors.InternalError("expected server.RunAs.FSGroup to be an int64: %w", err)
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

func (r boundEnv) deployEndpoint(ctx context.Context, server runtime.ServerConfig, endpoint *schema.Endpoint, s *serverRunState) error {
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

		s.declarations = append(s.declarations, kubedef.Apply{
			Description: fmt.Sprintf("Service %s", endpoint.ServiceName),
			Resource:    "services",
			Namespace:   ns,
			Name:        endpoint.AllocatedName,
			Body: applycorev1.
				Service(endpoint.AllocatedName, ns).
				WithLabels(kubedef.MakeServiceLabels(r.host.Env, t.Proto(), endpoint)).
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
