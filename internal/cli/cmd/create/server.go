// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"
	"text/template"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/languages/cue"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/go-ids"
)

const (
	nodeFileName = "server.cue"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Creates a server.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			if loc.RelPath == "." {
				return fmt.Errorf("Cannot create server at workspace root. Please specify server location or run %s at the target directory.", colors.Bold("fn create server"))
			}

			opts := serverTmplOptions{
				Id: ids.NewRandomBase32ID(12),
			}

			return cue.GenerateCueSource(ctx, root.FS(), loc.Rel(nodeFileName), serverTmpl, opts)

		}),
	}

	return cmd
}

type serverTmplOptions struct {
	Id        string
	Name      string
	Framework string
}

var serverTmpl = template.Must(template.New(nodeFileName).Parse(`
import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "{{.Id}}"
	name:      "{{.Name}}"
	framework: "{{.Framework}}"

	import: [
		{{if eq .Framework "GO_GRPC"}}
		// To expose GRPC endpoints via HTTP, add this import: 
		// "namespacelabs.dev/foundation/std/go/grpc/gateway",
		{{end}}

		// TODO add services here
	]
}
`))
