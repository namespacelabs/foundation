// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/providers/nscloud/metadata"
)

func newInlineSpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inline-spec",
		Short: "Convert the current token into an inline token spec.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			token, err := fnapi.FetchToken(ctx)
			if err != nil {
				return err
			}

			spec := metadata.MetadataSpec{
				InlineToken: token.Raw(),
			}

			data, err := json.Marshal(spec)
			if err != nil {
				return err
			}

			encoded := base64.RawStdEncoding.EncodeToString(data)

			fmt.Fprintf(console.Stdout(ctx), "Use NSC_TOKEN_SPEC=%s to set the current token inline.\n", encoded)
			return nil
		}),
	}

	return cmd
}
