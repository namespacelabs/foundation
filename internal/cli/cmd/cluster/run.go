// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
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
}

type exportContainerPort struct {
	ContainerPort    int32
	HttpIngressRules []*api.ContainerPort_HttpMatchRule
}

type CreateContainerResult struct {
	InstanceId  string
	InstanceUrl string
	ApiEndpoint string

	LegacyContainer []*api.Container
}

func CreateContainerInstance(ctx context.Context, machineType string, duration, waitFor time.Duration, target string, opts CreateContainerOpts) (*CreateContainerResult, error) {
	container := &api.ContainerRequest{
		Name:         opts.Name,
		Image:        opts.Image,
		Args:         opts.Args,
		Env:          opts.Env,
		Flag:         []string{"TERMINATE_ON_EXIT"},
		Network:      opts.Network,
		Experimental: opts.Experimental,
	}

	if opts.EnableDocker {
		container.DockerSockPath = "/var/run/docker.sock"
	}

	if opts.ForwardNscState {
		container.NscStatePath = "/var/run/nsc"
	}

	if opts.ExposeNscBins {
		container.ExposeNscBins = "/nsc/bin"
	}

	for _, port := range opts.ExportedPorts {
		container.ExportPort = append(container.ExportPort, &api.ContainerPort{
			Proto:         "tcp",
			Port:          port.ContainerPort,
			HttpMatchRule: port.HttpIngressRules,
		})
	}

	var labels []*api.LabelEntry
	for key, value := range opts.Labels {
		labels = append(labels, &api.LabelEntry{Name: key, Value: value})
	}

	if target == "" {
		const label = "Creating container environment"

		resp, err := tasks.Return(ctx, tasks.Action("nscloud.create-containers").HumanReadable(label), func(ctx context.Context) (*CreateContainerResult, error) {
			req := api.CreateInstanceRequest{
				MachineType:  machineType,
				Container:    []*api.ContainerRequest{container},
				Label:        labels,
				Feature:      opts.Features,
				Experimental: opts.InstanceExperimental,
			}

			if duration > 0 {
				dl := time.Now().Add(duration)
				req.Deadline = &dl
			}

			var response api.CreateInstanceResponse
			if err := api.Methods.CreateInstance.Do(ctx, req, endpoint.ResolveRegionalEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
				return nil, err
			}

			return &CreateContainerResult{
				InstanceId:  response.InstanceId,
				InstanceUrl: response.InstanceUrl,
				ApiEndpoint: response.ApiEndpoint,
			}, nil
		})
		if err != nil {
			return nil, err
		}

		if _, err := api.WaitClusterReady(ctx, api.Methods, resp.InstanceId, waitFor, api.WaitClusterOpts{
			ApiEndpoint: resp.ApiEndpoint,
			CreateLabel: label,
		}); err != nil {
			return nil, err
		}

		return resp, nil
	}

	res, err := api.EnsureCluster(ctx, api.Methods, nil, target)
	if err != nil {
		return nil, err
	}

	return tasks.Return(ctx, tasks.Action("nscloud.start-containers").HumanReadable("Starting containers"),
		func(ctx context.Context) (*CreateContainerResult, error) {
			var response api.StartContainersResponse
			if err := api.Methods.StartContainers.Do(ctx, api.StartContainersRequest{
				Id:        target,
				Container: []*api.ContainerRequest{container},
			}, api.MaybeEndpoint(res.Cluster.ApiEndpoint), fnapi.DecodeJSONResponse(&response)); err != nil {
				return nil, err
			}

			return &CreateContainerResult{
				LegacyContainer: response.Container,
			}, nil
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
		if err := d.Encode(createOutput{
			ClusterId:  resp.InstanceId,
			ClusterUrl: resp.InstanceUrl,
			Container:  resp.LegacyContainer,
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
