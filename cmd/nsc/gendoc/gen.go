// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	cobradoc "github.com/spf13/cobra/doc"
	nsccmd "namespacelabs.dev/foundation/cmd/nsc/cmd"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Syntax: gendoc <output dir>")
	}
	outDir := os.Args[1]

	root := &cobra.Command{Use: "nsc"}
	nsccmd.RegisterCommands(root)
	if err := cobradoc.GenMarkdownTree(root, outDir); err != nil {
		log.Fatal(err)
	}
}
