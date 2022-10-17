// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
)

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "info",
	}

	flag := cmd.Flags()
	workspaceDir := flag.String("workspace", ".", "The workspace directory to parse.")
	depVersion := flag.String("dep_version", "", "Print the version of a given dependency only.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		data, err := parsing.ModuleAt(ctx, *workspaceDir, parsing.ModuleAtArgs{SkipAPIRequirements: true})
		if err != nil {
			return err
		}

		w := data.Proto()

		output := infoOutput{
			ModuleName: w.ModuleName,
		}

		if len(w.Dep) > 0 {
			output.Dependencies = map[string]*schema.Workspace_Dependency{}
			for _, dep := range w.Dep {
				output.Dependencies[dep.ModuleName] = dep
			}
		}

		if *depVersion != "" {
			if dep, ok := output.Dependencies[*depVersion]; ok && dep.Version != "" {
				fmt.Println(dep.Version)
			} else {
				fmt.Println("HEAD")
			}
			return nil
		}

		output.BuildkitVersion, err = buildkit.Version()
		if err != nil {
			return err
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	})

	return cmd
}

type infoOutput struct {
	ModuleName      string                                  `json:"moduleName"`
	Dependencies    map[string]*schema.Workspace_Dependency `json:"dependencies,omitempty"`
	BuildkitVersion string                                  `json:"buildkitVersion"`
}
