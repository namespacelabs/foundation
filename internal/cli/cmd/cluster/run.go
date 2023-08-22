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

	composecli "github.com/compose-spec/compose-go/cli"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
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
	on := run.Flags().String("on", "", "Run the container in the specified container, instead of creating a new one.")
	env := run.Flags().StringToStringP("env", "e", map[string]string{}, "Pass these additional environment variables to the container.")
	devmode := run.Flags().Bool("development", false, "If true, enables a few development facilities, including making containers optional.")
	wait := run.Flags().Bool("wait", false, "Wait for the container to start running.")
	features := run.Flags().StringSlice("features", nil, "A set of features to attach to the cluster.")
	ingressRules := run.Flags().StringToString("ingress", map[string]string{}, "Specify ingress rules for ports; specify * to apply rules to any port; separate each rule with ;.")
	duration := run.Flags().Duration("duration", 0, "For how long to run the ephemeral environment.")
	labels := run.Flags().StringToString("label", nil, "Create the environment with a set of labels.")
	internalExtra := run.Flags().String("internal_extra", "", "Internal creation details.")
	enableDocker := run.Flags().Bool("enable_docker", false, "If set to true, instructs the platform to also setup docker in the container.")
	forwardNscState := run.Flags().Bool("forward_nsc_state", false, "If set to true, instructs the platform to forward nsc state into the container.")
	network := run.Flags().String("network", "", "The network setting to start the container with.")

	run.Flags().MarkHidden("label")
	run.Flags().MarkHidden("internal_extra")
	run.Flags().MarkHidden("enable_docker")
	run.Flags().MarkHidden("forward_nsc_state")
	run.Flags().MarkHidden("network")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		name := *requestedName

		if *image == "" {
			return fnerrors.New("--image is required")
		}

		if name == "" {
			name = generateNameFromImage(*image)
		}

		if *on != "" {
			if *devmode {
				return fnerrors.New("--development can only be set when creating an environment (i.e. it can't be set when --on is specified)")
			}

			if *machineType != "" {
				return fnerrors.New("--machine_type can only be set when creating an environment (i.e. it can't be set when --on is specified)")
			}

			if *duration > 0 {
				return fnerrors.New("--duration can only be set when creating an environment (i.e. it can't be set when --on is specified)")
			}
		}

		opts := createContainerOpts{
			Name:            name,
			Image:           *image,
			Args:            args,
			Env:             *env,
			Features:        *features,
			Labels:          *labels,
			InternalExtra:   *internalExtra,
			EnableDocker:    *enableDocker,
			ForwardNscState: *forwardNscState,
			Network:         *network,
		}

		exported, err := fillInIngressRules(*exportedPorts, *ingressRules)
		if err != nil {
			return err
		}

		opts.ExportedPorts = exported

		resp, err := createContainer(ctx, *machineType, *duration, *on, *devmode, opts)
		if err != nil {
			return err
		}

		if *wait {
			clusterId := *on
			if clusterId == "" {
				clusterId = resp.ClusterId
			}
			if err := ctl.WaitContainers(ctx, clusterId, resp.Container); err != nil {
				return err
			}
		}

		// This needs to handle the case both of when a cluster is created, and
		// when StartContainers are called.
		return printResult(ctx, *output, resp)
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

		return nil, fnerrors.New("specified ingress rule for port %q which is not exported", k)
	}

	return exported, nil
}

func NewRunComposeCmd() *cobra.Command {
	run := &cobra.Command{
		Use:    "run-compose",
		Short:  "Starts a set of containers in an ephemeral environment.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	output := run.Flags().StringP("output", "o", "plain", "one of plain or json")
	dir := run.Flags().String("dir", "", "If not specified, loads the compose project from the current working directory.")
	devmode := run.Flags().Bool("development", false, "If true, enables a few development facilities, including making containers optional.")
	wait := run.Flags().Bool("wait", false, "Wait for all containers to start running.")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		resp, err := createCompose(ctx, *dir, *devmode)
		if err != nil {
			return err
		}

		if *wait {
			if err := ctl.WaitContainers(ctx, resp.ClusterId, resp.Container); err != nil {
				return err
			}
		}

		return printResult(ctx, *output, resp)
	})

	return run
}

type createContainerOpts struct {
	Name            string
	Image           string
	Args            []string
	Env             map[string]string
	Flags           []string
	ExportedPorts   []exportContainerPort
	Features        []string
	Labels          map[string]string
	InternalExtra   string
	EnableDocker    bool
	ForwardNscState bool
	Network         string
}

type exportContainerPort struct {
	ContainerPort    int32
	HttpIngressRules []*api.ContainerPort_HttpMatchRule
}

func createContainer(ctx context.Context, machineType string, duration time.Duration, target string, devmode bool, opts createContainerOpts) (*api.CreateContainersResponse, error) {
	container := &api.ContainerRequest{
		Name:    opts.Name,
		Image:   opts.Image,
		Args:    opts.Args,
		Env:     opts.Env,
		Flag:    []string{"TERMINATE_ON_EXIT"},
		Network: opts.Network,
	}

	if opts.EnableDocker {
		container.DockerSockPath = "/var/run/docker.sock"
	}

	if opts.ForwardNscState {
		container.NscStatePath = "/var/run/nsc"
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

		resp, err := tasks.Return(ctx, tasks.Action("nscloud.create-containers").HumanReadablef(label), func(ctx context.Context) (*api.CreateContainersResponse, error) {
			req := api.CreateContainersRequest{
				MachineType:     machineType,
				Container:       []*api.ContainerRequest{container},
				DevelopmentMode: devmode,
				Label:           labels,
				Feature:         opts.Features,
				InternalExtra:   opts.InternalExtra,
			}

			if duration > 0 {
				req.Deadline = timestamppb.New(time.Now().Add(duration))
			}

			var response api.CreateContainersResponse
			if err := api.Methods.CreateContainers.Do(ctx, req, api.ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
				return nil, err
			}
			return &response, nil
		})
		if err != nil {
			return nil, err
		}

		if _, err := api.WaitCluster(ctx, api.Methods, resp.ClusterId, api.WaitClusterOpts{
			CreateLabel: label,
		}); err != nil {
			return nil, err
		}

		return resp, nil
	}

	return tasks.Return(ctx, tasks.Action("nscloud.start-containers").HumanReadablef("Starting containers"),
		func(ctx context.Context) (*api.CreateContainersResponse, error) {
			var response api.StartContainersResponse
			if err := api.Methods.StartContainers.Do(ctx, api.StartContainersRequest{
				Id:        target,
				Container: []*api.ContainerRequest{container},
			}, api.ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
				return nil, err
			}

			return &api.CreateContainersResponse{
				Container: response.Container,
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
			return nil, fnerrors.New("unrecognized rule %q", p)
		}
	}

	return x, nil
}

func printResult(ctx context.Context, output string, resp *api.CreateContainersResponse) error {
	switch output {
	case "json":
		d := json.NewEncoder(console.Stdout(ctx))
		d.SetIndent("", "  ")
		return d.Encode(createOutput{
			ClusterId:  resp.ClusterId,
			ClusterUrl: resp.ClusterUrl,
			Container:  resp.Container,
		})

	default:
		if output != "" && output != "plain" {
			fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
		}

		// ClusterId is not set when `--on` is used.
		if resp.ClusterId != "" {
			printNewEnv(ctx, resp.ClusterId, resp.ClusterUrl)
		}

		for _, ctr := range resp.Container {
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

func createCompose(ctx context.Context, dir string, devmode bool) (*api.CreateContainersResponse, error) {
	var optionsFn []composecli.ProjectOptionsFn
	optionsFn = append(optionsFn,
		composecli.WithOsEnv,
		// composecli.WithEnvFile(o.EnvFile),
		composecli.WithConfigFileEnv,
		composecli.WithDefaultConfigPath,
		composecli.WithDotEnv,
		// composecli.WithName(o.Project),
	)

	if dir != "" {
		optionsFn = append(optionsFn, composecli.WithWorkingDirectory(dir))
	}

	projectOptions, err := composecli.NewProjectOptions(nil, optionsFn...)
	if err != nil {
		return nil, err
	}

	project, err := composecli.ProjectFromOptions(projectOptions)
	if err != nil {
		return nil, err
	}

	projectYAML, err := yaml.Marshal(project)
	if err != nil {
		return nil, err
	}

	resp, err := tasks.Return(ctx, tasks.Action("nscloud.create-containers"), func(ctx context.Context) (*api.CreateContainersResponse, error) {
		var response api.CreateContainersResponse
		if err := api.Methods.CreateContainers.Do(ctx, api.CreateContainersRequest{
			Compose:         []*api.ComposeRequest{{Contents: projectYAML}},
			DevelopmentMode: devmode,
		}, api.ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
	if err != nil {
		return nil, err
	}

	if _, err := api.WaitCluster(ctx, api.Methods, resp.ClusterId, api.WaitClusterOpts{}); err != nil {
		return nil, err
	}

	return resp, nil
}

func sshRun(ctx context.Context, sshcli *ssh.Client, io rtypes.IO, cmd string) error {
	sess, err := sshcli.NewSession()
	if err != nil {
		return err
	}

	defer sess.Close()

	sess.Stdin = io.Stdin
	sess.Stdout = io.Stdout
	sess.Stderr = io.Stderr

	return sess.Run(cmd)
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
}
