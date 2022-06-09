// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/schema"
)

var workspaceDir = flag.String("workspace", ".", "The workspace directory to parse.")
var depVersion = flag.String("dep_version", "", "Print the version of a given dependency only.")

func main() {
	flag.Parse()

	if err := Do(context.Background(), *workspaceDir); err != nil {
		log.Fatal(err)
	}
}

type Output struct {
	ModuleName      string                                  `json:"moduleName"`
	Dependencies    map[string]*schema.Workspace_Dependency `json:"dependencies,omitempty"`
	BuildkitVersion string                                  `json:"buildkitVersion"`
}

func Do(ctx context.Context, workspaceDir string) error {
	data, err := cuefrontend.ModuleLoader.ModuleAt(ctx, workspaceDir)
	if err != nil {
		return err
	}

	w := data.Parsed()

	output := Output{
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
}
