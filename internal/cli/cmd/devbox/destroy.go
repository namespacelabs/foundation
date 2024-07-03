// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"fmt"
	"strings"

	devboxv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/private/devbox"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/private"
)

func newDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy tag...",
		Short: "Destroys devboxes with the specified tags.",
		Args:  cobra.MinimumNArgs(1),
	}

	force := cmd.Flags().Bool("force", false, "Skip the confirmation step.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return destroyDevbox(ctx, args, *force)
	})

	return cmd
}

func destroyDevbox(ctx context.Context, tags []string, force bool) error {
	devboxClient, err := getDevBoxClient(ctx)
	if err != nil {
		return err
	}

	// Don't do anything if one of the tags is invalid.
	if err := checkAllExist(ctx, devboxClient, tags); err != nil {
		return err
	}

	for _, tag := range tags {
		if !force {
			result, err := tui.Ask(ctx, "Do you want to remove this devbox?",
				fmt.Sprintf(`This is a destructive action.

	Type %q for it to be removed.`, tag), "")
			if err != nil {
				return err
			}

			if result != tag {
				return context.Canceled
			}
		}

		_, err := devboxClient.DeleteDevBox(ctx, &devboxv1beta.DeleteDevBoxRequest{
			DevboxTag: tag,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func checkAllExist(ctx context.Context, devboxClient *private.DevBoxServiceClient, tags []string) error {
	missing := map[string]struct{}{}
	for _, tag := range tags {
		missing[tag] = struct{}{}
	}

	resp, err := devboxClient.ListDevBoxes(ctx, &devboxv1beta.ListDevBoxesRequest{
		TagFilter: tags,
	})
	if err != nil {
		return err
	}

	for _, found := range resp.DevBoxes {
		delete(missing, found.GetDevboxSpec().GetTag())
	}

	if len(tags) == 1 {
		return fmt.Errorf("devbox '" + tags[0] + "' not found")
	} else if len(tags) > 1 {
		missingSlice := make([]string, 0, len(missing))
		for missingTag, _ := range missing {
			missingSlice = append(missingSlice, missingTag)
		}
		return fmt.Errorf("devboxes with tags " + strings.Join(missingSlice, ", ") + " not found")
	}
	return nil
}
