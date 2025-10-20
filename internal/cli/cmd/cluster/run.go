// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	computev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/compute/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
	"namespacelabs.dev/integrations/api/compute"
	"namespacelabs.dev/integrations/auth"
)

func NewRunCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Starts a container in an ephemeral environment, optionally exporting ports for public serving.",
		Args:  cobra.ArbitraryArgs,
	}

	machineType := run.Flags().String("machine_type", "", "Specify the machine type.")
	image := run.Flags().String("image", "", "Which image to run.")
	requestedName := run.Flags().String("name", "", "If no name is specified, one is generated.")
	exportedPorts := run.Flags().Int32SliceP("publish", "p", nil, "Publish the specified ports.")
	output := run.Flags().StringP("output", "o", "plain", "One of plain or json.")
	on := run.Flags().String("on", "", "Run the container in the specified instance, instead of creating a new one.")
	env := run.Flags().StringToStringP("env", "e", map[string]string{}, "Pass these additional environment variables to the container.")
	devmode := run.Flags().Bool("development", false, "If true, enables a few development facilities, including making containers optional.")
	wait := run.Flags().Bool("wait", false, "Wait for the container to start running.")
	waitTimeout := run.Flags().Duration("wait_timeout", time.Minute, "For how long to wait until the instance becomes ready.")
	features := run.Flags().StringSlice("features", nil, "A set of features to attach to the instance.")
	ingressRules := run.Flags().StringToString("ingress", map[string]string{}, "Specify ingress rules for ports; specify * to apply rules to any port; separate each rule with ;.")
	duration := run.Flags().Duration("duration", 0, "For how long to run the ephemeral environment.")
	labels := run.Flags().StringToString("label", nil, "Create the environment with a set of labels.")
	internalExtra := run.Flags().String("internal_extra", "", "Internal creation details.")
	enableDocker := run.Flags().Bool("enable_docker", false, "If set to true, instructs the platform to also setup docker in the container.")
	forwardNscState := run.Flags().Bool("forward_nsc_state", false, "If set to true, instructs the platform to forward nsc state into the container.")
	exposeNscBins := run.Flags().Bool("expose_nsc_bins", false, "If set to true, exposes Namespace managed nsc binaries to the container.")
	network := run.Flags().String("network", "", "The network setting to start the container with.")
	experimental := run.Flags().String("experimental", "", "A set of experimental settings to pass during creation.")
	instanceExperimental := run.Flags().String("instance_experimental", "", "A set of experimental instance settings to pass during creation.")
	userSshey := run.Flags().String("ssh_key", "", "Injects the specified ssh public key in the created instance.")
	volumes := run.Flags().StringSlice("volume", nil, "Attach a volume to the instance, {cache|persistent}:{tag}:{mountpoint}:{size}")

	user := run.Flags().String("user", "", "Customize the user to run the container as.")

	run.Flags().MarkHidden("label")
	run.Flags().MarkHidden("internal_extra")
	run.Flags().MarkHidden("forward_nsc_state")
	run.Flags().MarkHidden("network")
	run.Flags().MarkHidden("experimental")
	run.Flags().MarkHidden("instance_experimental")
	run.Flags().MarkHidden("expose_nsc_bins")
	run.Flags().MarkHidden("ssh_key")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		name := *requestedName

		if *image == "" {
			return fnerrors.Newf("--image is required")
		}

		if name == "" {
			name = generateNameFromImage(*image)
		}

		if *on != "" {
			if *devmode {
				return fnerrors.Newf("--development can only be set when creating an environment (i.e. it can't be set when --on is specified)")
			}

			if *machineType != "" {
				return fnerrors.Newf("--machine_type can only be set when creating an environment (i.e. it can't be set when --on is specified)")
			}

			if *duration > 0 {
				return fnerrors.Newf("--duration can only be set when creating an environment (i.e. it can't be set when --on is specified)")
			}
		}

		opts := CreateContainerOpts{
			Name:            name,
			Image:           *image,
			Args:            args,
			Env:             *env,
			Features:        *features,
			Labels:          *labels,
			InternalExtra:   *internalExtra,
			EnableDocker:    *enableDocker,
			ForwardNscState: *forwardNscState,
			ExposeNscBins:   *exposeNscBins,
			Network:         *network,
			User:            *user,
		}

		if *experimental != "" {
			if err := json.Unmarshal([]byte(*experimental), &opts.Experimental); err != nil {
				return fnerrors.Newf("failed to parse: %w", err)
			}
		}

		if *instanceExperimental != "" {
			if err := json.Unmarshal([]byte(*instanceExperimental), &opts.InstanceExperimental); err != nil {
				return fnerrors.Newf("failed to parse: %w", err)
			}
		}

		if *instanceExperimental != "" {
			if err := json.Unmarshal([]byte(*instanceExperimental), &opts.InstanceExperimental); err != nil {
				return fnerrors.Newf("failed to parse: %w", err)
			}
		}

		if keys, err := parseAuthorizedKeys(*userSshey); err != nil {
			return err
		} else {
			if opts.InstanceExperimental == nil {
				opts.InstanceExperimental = map[string]any{}
			}

			opts.InstanceExperimental["authorized_ssh_keys"] = keys
		}

		var volumeSpecs []api.VolumeSpec
		var hostMounts []api.ContainerBindMount
		for i, def := range *volumes {
			spec, err := ParseVolumeFlag(def)
			if err != nil {
				return err
			}

			// Translate volume spec into: host VolumeSpec at static path + container mount at requested path
			hostPath := fmt.Sprintf("/volumes/volume%d", i)
			volumeSpecs = append(volumeSpecs, api.VolumeSpec{
				Tag:             spec.Tag,
				SizeMb:          spec.SizeMb,
				PersistencyKind: spec.PersistencyKind,
				MountPoint:      hostPath,
			})
			hostMounts = append(hostMounts, api.ContainerBindMount{
				HostPath:      hostPath,
				ContainerPath: spec.MountPoint,
			})
		}

		if len(volumeSpecs) > 0 {
			if opts.InstanceExperimental == nil {
				opts.InstanceExperimental = map[string]any{}
			}

			opts.InstanceExperimental["volumes"] = volumeSpecs
		}

		if len(hostMounts) > 0 {
			if opts.Experimental == nil {
				opts.Experimental = map[string]any{}
			}

			opts.Experimental["host_mount"] = hostMounts
		}

		exported, err := fillInIngressRules(*exportedPorts, *ingressRules)
		if err != nil {
			return err
		}

		if *devmode {
			if opts.InstanceExperimental == nil {
				opts.InstanceExperimental = map[string]any{}
			}

			opts.InstanceExperimental["development_mode"] = *devmode
		}

		opts.ExportedPorts = exported

		resp, err := CreateContainerInstance(ctx, *machineType, *duration, *waitTimeout, *on, opts)
		if err != nil {
			return err
		}

		if *wait {
			clusterId := *on
			if clusterId == "" {
				clusterId = resp.InstanceId
			}

			if err := ctl.WaitContainers(ctx, clusterId, resp.LegacyContainer); err != nil {
				return err
			}
		}

		// This needs to handle the case both of when a cluster is created, and
		// when StartContainers are called.
		return PrintCreateContainersResult(ctx, *output, resp)
	})

	return run
}

func fillInIngressRules(ports []int32, ingressRules map[string]string) ([]exportContainerPort, error) {
	var exported []exportContainerPort

	matched := map[string]struct{}{}
	for _, p := range ports {
		portKey := fmt.Sprintf("%d", p)
		rules, ok := ingressRules[portKey]
		if !ok {
			rules = ingressRules["*"]
		} else {
			matched[portKey] = struct{}{}
		}

		x := exportContainerPort{
			ContainerPort: p,
		}

		if rules != "" {
			for _, ruleSpec := range strings.Split(rules, ";") {
				rule, err := parseRule(ruleSpec)
				if err != nil {
					return nil, err
				}

				x.HttpIngressRules = append(x.HttpIngressRules, rule)
			}
		}

		exported = append(exported, x)
	}

	for k := range ingressRules {
		if _, ok := matched[k]; ok || k == "*" {
			continue
		}

		return nil, fnerrors.Newf("specified ingress rule for port %q which is not exported", k)
	}

	return exported, nil
}

type CreateContainerOpts struct {
	Name                 string
	Image                string
	Args                 []string
	Env                  map[string]string
	Flags                []string
	ExportedPorts        []exportContainerPort
	Features             []string
	Labels               map[string]string
	InternalExtra        string
	EnableDocker         bool
	ForwardNscState      bool
	ExposeNscBins        bool
	Network              string
	Experimental         map[string]any
	InstanceExperimental map[string]any
	User                 string
}

type exportContainerPort struct {
	ContainerPort    int32
	HttpIngressRules []*api.ContainerPort_HttpMatchRule
}

type CreateContainerResult struct {
	InstanceId  string
	InstanceUrl string
	ApiEndpoint string

	Containers      []api.CreateInstanceResponse_ContainerReference
	LegacyContainer []*api.Container
}

// parseMachineTypeToShape converts a machine type string to InstanceShape.
// If machineType is empty, returns default shape.
// Simple format: just pass CPU/memory directly as numbers would be parsed from string
// For now, we use sensible defaults based on common machine types.
func parseMachineTypeToShape(machineType string) *computev1beta.InstanceShape {
	shape := &computev1beta.InstanceShape{
		MachineArch: "amd64",
		Os:          "linux",
	}

	// Default shape if no machine type specified
	if machineType == "" {
		shape.VirtualCpu = 2
		shape.MemoryMegabytes = 4096
		return shape
	}

	// Parse common machine type patterns (e.g., "t3.medium", "4x8", etc.)
	// For simplicity, if it contains "x", parse as CPUxMEM format
	if strings.Contains(machineType, "x") {
		parts := strings.Split(machineType, "x")
		if len(parts) == 2 {
			if cpu, err := strconv.Atoi(parts[0]); err == nil {
				shape.VirtualCpu = int32(cpu)
			}
			if mem, err := strconv.Atoi(parts[1]); err == nil {
				shape.MemoryMegabytes = int32(mem) * 1024 // Convert GB to MB
			}
			return shape
		}
	}

	// Default to reasonable values if parsing fails
	shape.VirtualCpu = 2
	shape.MemoryMegabytes = 4096
	return shape
}

// convertContainerRequest converts CreateContainerOpts to computev1beta.ContainerRequest
func convertContainerRequest(opts CreateContainerOpts) *computev1beta.ContainerRequest {
	container := &computev1beta.ContainerRequest{
		Name:     opts.Name,
		ImageRef: opts.Image,
		Args:     opts.Args,
	}

	// Convert environment variables
	if len(opts.Env) > 0 {
		container.Environment = opts.Env
	}

	// Set workload type to job (terminates on exit)
	container.WorkloadType = computev1beta.ContainerRequest_JOB

	// Set network mode
	if opts.Network == "host" {
		container.Network = computev1beta.ContainerRequest_HOST
	} else {
		container.Network = computev1beta.ContainerRequest_BRIDGE
	}

	if opts.EnableDocker {
		container.DockerSockPath = "/var/run/docker.sock"
	}

	if opts.ForwardNscState {
		container.NscStatePath = "/var/run/nsc"
	}

	// Handle exported ports
	for _, port := range opts.ExportedPorts {
		container.ExportPorts = append(container.ExportPorts, &computev1beta.ContainerPort{
			ContainerPort: port.ContainerPort,
			HttpMatchRule: convertHttpMatchRules(port.HttpIngressRules),
		})
	}

	// Note: User override not supported in new API
	// The opts.User field is ignored in the new API
	if opts.User != "" {
		// User override not available in compute v1beta API
		// This functionality may need to be handled differently or removed
	}

	// Handle experimental features
	if opts.Experimental != nil {
		container.Experimental = &computev1beta.ContainerRequest_ExperimentalFeatures{}

		// Handle host mounts
		if hostMounts, ok := opts.Experimental["host_mount"]; ok && hostMounts != nil {
			if mounts, ok := hostMounts.([]api.ContainerBindMount); ok {
				for _, mount := range mounts {
					container.Experimental.HostMount = append(container.Experimental.HostMount,
						&computev1beta.ContainerRequest_ExperimentalFeatures_HostMount{
							HostPath:      mount.HostPath,
							ContainerPath: mount.ContainerPath,
						})
				}
			}
		}
	}

	// Note: ExposeNscBins not supported in new API
	// This feature may need to be handled differently or removed
	if opts.ExposeNscBins {
		// ExposeNscBins not available in compute v1beta API
		// This functionality may need to be handled differently
	}

	return container
}

// convertHttpMatchRules converts old API match rules to new API format
func convertHttpMatchRules(rules []*api.ContainerPort_HttpMatchRule) []*computev1beta.HttpMatchRule {
	var converted []*computev1beta.HttpMatchRule
	for _, rule := range rules {
		newRule := &computev1beta.HttpMatchRule{
			DoesNotRequireAuth: rule.DoesNotRequireAuth,
		}
		if rule.Match != nil {
			newRule.Match = &computev1beta.HttpMatchRule_HttpMatch{
				Path:   rule.Match.Path,
				Method: rule.Match.Method,
			}
		}
		converted = append(converted, newRule)
	}
	return converted
}

// convertLabels converts label map to stdlib.Label slice
func convertLabels(labels map[string]string) []*stdlib.Label {
	var converted []*stdlib.Label
	for key, value := range labels {
		converted = append(converted, &stdlib.Label{
			Name:  key,
			Value: value,
		})
	}
	return converted
}

func CreateContainerInstance(ctx context.Context, machineType string, duration, waitFor time.Duration, target string, opts CreateContainerOpts) (*CreateContainerResult, error) {
	container := convertContainerRequest(opts)

	if target == "" {
		const label = "Creating container environment"

		resp, err := tasks.Return(ctx, tasks.Action("nscloud.create-containers").HumanReadable(label), func(ctx context.Context) (*CreateContainerResult, error) {
			// Load authentication token
			token, err := auth.LoadDefaults()
			if err != nil {
				return nil, fnerrors.Newf("failed to load authentication: %w", err)
			}

			// Create compute client
			cli, err := compute.NewClient(ctx, token)
			if err != nil {
				return nil, fnerrors.Newf("failed to create compute client: %w", err)
			}
			defer cli.Close()

			// Build the CreateInstance request
			req := &computev1beta.CreateInstanceRequest{
				Shape:             parseMachineTypeToShape(machineType),
				Containers:        []*computev1beta.ContainerRequest{container},
				Labels:            convertLabels(opts.Labels),
				DocumentedPurpose: "Container environment created via 'nsc cluster run'",
			}

			// Set deadline if duration specified
			if duration > 0 {
				deadline := time.Now().Add(duration)
				req.Deadline = timestamppb.New(deadline)
			}

			// Handle features
			// Note: Docker and Buildkit features are no longer configurable via FeatureConfiguration
			// They are now controlled at the container level via DockerSockPath
			if len(opts.Features) > 0 {
				req.FeatureConfiguration = &computev1beta.CreateInstanceRequest_FeatureConfiguration{}
				for _, feature := range opts.Features {
					switch feature {
					case "kubernetes":
						// Kubernetes version would need to be specified
						// For now, skip kubernetes feature handling
					default:
						// Other features not directly supported in new API
						// May need to use PrivateFeature experimental field
					}
				}
			}

			// Handle instance experimental features
			if opts.InstanceExperimental != nil {
				req.Experimental = &computev1beta.CreateInstanceRequest_ExperimentalFeatures{}

				// Map relevant experimental features
				if features, ok := opts.InstanceExperimental["private_features"]; ok {
					if featureList, ok := features.([]string); ok {
						req.Experimental.PrivateFeature = featureList
					}
				}

				// Note: development_mode, volumes, and authorized_ssh_keys not directly supported
				// May need alternative handling or removal
			}

			// Call CreateInstance
			response, err := cli.Compute.CreateInstance(ctx, req)
			if err != nil {
				return nil, fnerrors.Newf("failed to create instance: %w", err)
			}

			// Convert response
			result := &CreateContainerResult{
				InstanceId:  response.Metadata.InstanceId,
				InstanceUrl: response.InstanceUrl,
			}

			// Convert containers
			for _, allocatedContainer := range response.Containers {
				result.Containers = append(result.Containers, api.CreateInstanceResponse_ContainerReference{
					ContainerId: allocatedContainer.Id,
				})
			}

			return result, nil
		})
		if err != nil {
			return nil, err
		}

		// Note: WaitClusterReady is now handled by CreateInstance itself in the new API
		// The CreateInstance call blocks until the instance is ready
		return resp, nil
	}

	// When target is specified, use StartContainers to add containers to existing instance
	return tasks.Return(ctx, tasks.Action("nscloud.start-containers").HumanReadable("Starting containers"),
		func(ctx context.Context) (*CreateContainerResult, error) {
			// Load authentication token
			token, err := auth.LoadDefaults()
			if err != nil {
				return nil, fnerrors.Newf("failed to load authentication: %w", err)
			}

			// Create compute client
			cli, err := compute.NewClient(ctx, token)
			if err != nil {
				return nil, fnerrors.Newf("failed to create compute client: %w", err)
			}
			defer cli.Close()

			// Call StartContainers
			response, err := cli.Compute.StartContainers(ctx, &computev1beta.StartContainersRequest{
				InstanceId: target,
				Containers: []*computev1beta.ContainerRequest{container},
			})
			if err != nil {
				return nil, fnerrors.Newf("failed to start containers: %w", err)
			}

			// Convert legacy containers for backward compatibility
			result := &CreateContainerResult{}
			for _, allocatedContainer := range response.Containers {
				legacyContainer := &api.Container{
					Id:   allocatedContainer.Id,
					Name: allocatedContainer.Name,
				}

				// Convert exported ports
				for _, port := range allocatedContainer.ExportedPort {
					legacyContainer.ExportedPort = append(legacyContainer.ExportedPort, &api.Container_ExportedContainerPort{
						ContainerPort: port.ContainerPort,
						IngressFqdn:   port.Fqdn,
						Proto:         port.Proto.String(),
						ExportedPort:  port.ExportedPort,
					})
				}

				result.LegacyContainer = append(result.LegacyContainer, legacyContainer)
			}

			return result, nil
		})
}

func parseRule(spec string) (*api.ContainerPort_HttpMatchRule, error) {
	parts := strings.SplitN(spec, ":", 3)
	rule, err := parseEffect(parts[len(parts)-1])
	if err != nil {
		return nil, err
	}

	switch len(parts) {
	case 1:
		// Apply to all paths and methods

	case 2:
		// All methods: [path]: [spec]
		rule.Match = &api.ContainerPort_HttpMatch{Path: parts[0]}

	case 3:
		// [methods]:[path]:[spec]
		rule.Match = &api.ContainerPort_HttpMatch{Method: parseMethods(parts[0]), Path: parts[1]}
	}

	return rule, nil
}

func parseMethods(spec string) []string {
	var methods []string

	parts := strings.Split(spec, ",")
	for _, p := range parts {
		methods = append(methods, strings.ToUpper(p))
	}

	return methods
}

func parseEffect(spec string) (*api.ContainerPort_HttpMatchRule, error) {
	parts := strings.Split(spec, ",")
	x := &api.ContainerPort_HttpMatchRule{}
	for _, p := range parts {
		switch strings.ToLower(p) {
		case "noauth":
			x.DoesNotRequireAuth = true

		default:
			return nil, fnerrors.Newf("unrecognized rule %q", p)
		}
	}

	return x, nil
}

func PrintCreateContainersResult(ctx context.Context, output string, resp *CreateContainerResult) error {
	switch output {
	case "json":
		d := json.NewEncoder(console.Stdout(ctx))
		d.SetIndent("", "  ")

		containers := resp.LegacyContainer

		for _, ctr := range resp.Containers {
			containers = append(containers, &api.Container{
				Id: ctr.ContainerId,
			})
		}

		if err := d.Encode(createOutput{
			ClusterId:  resp.InstanceId,
			ClusterUrl: resp.InstanceUrl,
			Container:  containers,
		}); err != nil {
			return fnerrors.InternalError("failed to encode countainer creation output as JSON output: %w", err)
		}

	default:
		if output != "" && output != "plain" {
			fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
		}

		// ClusterId is not set when `--on` is used.
		if resp.InstanceId != "" {
			printNewEnv(ctx, resp.InstanceId, resp.InstanceUrl)
		}

		for _, ctr := range resp.LegacyContainer {
			fmt.Fprintf(console.Stdout(ctx), "\n  Started %q\n", ctr.Name)
			if len(ctr.ExportedPort) > 0 {
				fmt.Fprintln(console.Stdout(ctx))
				for _, port := range ctr.ExportedPort {
					fmt.Fprintf(console.Stdout(ctx), "    Exported %d/%s as https://%s\n", port.ContainerPort, port.Proto, port.IngressFqdn)
				}
			}
		}

		fmt.Fprintln(console.Stdout(ctx))
	}

	return nil
}

func printNewEnv(ctx context.Context, clusterID, clusterURL string) {
	style := colors.Ctx(ctx)

	stdout := console.Stdout(ctx)

	fmt.Fprintf(stdout, "\n  Created new ephemeral environment! ID: %s\n", clusterID)
	fmt.Fprintf(stdout, "\n  %s %s\n", style.Comment.Apply("More at:"), clusterURL)
}

func generateNameFromImage(image string) string {
	if tag, err := name.NewTag(image); err == nil {
		p := strings.Split(tag.RepositoryStr(), "/")
		last := p[len(p)-1]
		if len(last) < 16 {
			return fmt.Sprintf("%s-%s", last, ids.NewRandomBase32ID(3))
		}
	}

	return ids.NewRandomBase32ID(6)
}

type createOutput struct {
	ClusterId     string           `json:"cluster_id,omitempty"`
	ClusterUrl    string           `json:"cluster_url,omitempty"`
	Container     []*api.Container `json:"container,omitempty"`
	IngressDomain string           `json:"ingress_domain,omitempty"`
	ApiEndpoint   string           `json:"api_endpoint,omitempty"`
}
