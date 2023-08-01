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
