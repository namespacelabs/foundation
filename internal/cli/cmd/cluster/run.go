// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	composecli "github.com/compose-spec/compose-go/cli"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

func NewRunCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Starts a container in an ephemeral environment, optionally exporting ports for public serving.",
		Args:  cobra.NoArgs,
	}

	image := run.Flags().String("image", "", "Which image to run.")
	requestedName := run.Flags().String("name", "", "If no name is specified, one is generated.")
	exportedPorts := run.Flags().Int32SliceP("publish", "p", nil, "Publish the specified ports.")
	output := run.Flags().StringP("output", "o", "plain", "One of plain or json.")
	on := run.Flags().String("on", "", "Run the container in the specified container, instead of creating a new one.")
	env := run.Flags().StringToStringP("env", "e", map[string]string{}, "Pass these additional environment variables to the container.")
	devmode := run.Flags().Bool("development", false, "If true, enables a few development facilities, including making containers optional.")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		name := *requestedName

		if *image == "" {
			return fnerrors.New("--image is required")
		}

		if name == "" {
			name = generateNameFromImage(*image)
		}

		if *devmode && *on != "" {
			return fnerrors.New("--development can only be set when creating an environment (i.e. it can't be set when --on is specified)")
		}

		resp, err := createContainer(ctx, *on, *devmode, createContainerOpts{
			Name:          name,
			Image:         *image,
			ExportedPorts: *exportedPorts,
			Args:          args,
			Env:           *env,
		})
		if err != nil {
			return err
		}

		return printResult(ctx, *output, resp)
	})

	return run
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

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		resp, err := createCompose(ctx, *dir, *devmode)
		if err != nil {
			return err
		}

		return printResult(ctx, *output, resp)
	})

	return run
}

type createContainerOpts struct {
	Name          string
	Image         string
	Args          []string
	Env           map[string]string
	Flags         []string
	ExportedPorts []int32
}

func createContainer(ctx context.Context, target string, devmode bool, opts createContainerOpts) (*api.CreateContainersResponse, error) {
	container := &api.ContainerRequest{
		Name:  opts.Name,
		Image: opts.Image,
		Args:  opts.Args,
		Env:   opts.Env,
		Flag:  []string{"TERMINATE_ON_EXIT"},
	}

	for _, port := range opts.ExportedPorts {
		container.ExportPort = append(container.ExportPort, &api.ContainerPort{
			Proto: "tcp",
			Port:  port,
		})
	}

	if target == "" {
		resp, err := tasks.Return(ctx, tasks.Action("nscloud.create-containers"), func(ctx context.Context) (*api.CreateContainersResponse, error) {
			var response api.CreateContainersResponse
			if err := api.Endpoint.CreateContainers.Do(ctx, api.CreateContainersRequest{
				Container:       []*api.ContainerRequest{container},
				DevelopmentMode: devmode,
			}, fnapi.DecodeJSONResponse(&response)); err != nil {
				return nil, err
			}
			return &response, nil
		})
		if err != nil {
			return nil, err
		}

		if _, err := api.WaitCluster(ctx, api.Endpoint, resp.ClusterId, api.WaitClusterOpts{}); err != nil {
			return nil, err
		}

		return resp, nil
	}

	return tasks.Return(ctx, tasks.Action("nscloud.start-containers"), func(ctx context.Context) (*api.CreateContainersResponse, error) {
		var response api.StartContainersResponse
		if err := api.Endpoint.StartContainers.Do(ctx, api.StartContainersRequest{
			Id:        target,
			Container: []*api.ContainerRequest{container},
		}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}

		return &api.CreateContainersResponse{
			Container: response.Container,
		}, nil
	})
}

func printResult(ctx context.Context, output string, resp *api.CreateContainersResponse) error {
	switch output {
	case "json":
		d := json.NewEncoder(console.Stdout(ctx))
		d.SetIndent("", "  ")
		return d.Encode(resp)

	default:
		if output != "" && output != "plain" {
			fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
		}

		if resp.ClusterId != "" {
			fmt.Fprintf(console.Stdout(ctx), "\n  Created new ephemeral environment! (id: %s).\n", resp.ClusterId)
			fmt.Fprintf(console.Stdout(ctx), "\n  More at: %s\n", resp.ClusterUrl)
		}

		for _, ctr := range resp.Container {
			fmt.Fprintf(console.Stdout(ctx), "\n  Running %q\n", ctr.Name)
			if len(ctr.ExportedPort) > 0 {
				fmt.Fprintln(console.Stdout(ctx))
				for _, port := range ctr.ExportedPort {
					fmt.Fprintf(console.Stdout(ctx), "    Exported %d/%s as https://%s\n", port.Port, port.Proto, port.IngressFqdn)
				}
			}
		}

		fmt.Fprintln(console.Stdout(ctx))
	}

	return nil
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
		if err := api.Endpoint.CreateContainers.Do(ctx, api.CreateContainersRequest{
			Compose:         []*api.ComposeRequest{{Contents: projectYAML}},
			DevelopmentMode: devmode,
		}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
	if err != nil {
		return nil, err
	}

	if _, err := api.WaitCluster(ctx, api.Endpoint, resp.ClusterId, api.WaitClusterOpts{}); err != nil {
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
