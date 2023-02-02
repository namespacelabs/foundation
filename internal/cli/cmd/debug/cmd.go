// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"github.com/spf13/cobra"
)

func NewDebugCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "debug",
		Short:   "Internal commands, for debugging.",
		Aliases: []string{"d", "dbg"},
	}

	cmd.AddCommand(newPrintComputedCmd())
	cmd.AddCommand(newComputeConfigCmd())
	cmd.AddCommand(newPrintSealedCmd())
	cmd.AddCommand(newImageIndexCmd())
	cmd.AddCommand(newImageCmd())
	cmd.AddCommand(newActionDemoCmd())
	cmd.AddCommand(newDownloadCmd())
	cmd.AddCommand(newPrepareCmd())
	cmd.AddCommand(newDnsQuery())
	cmd.AddCommand(newDecodeProtoCmd())
	cmd.AddCommand(newUpdateLicenseCmd())
	cmd.AddCommand(newKubernetesCmd())
	cmd.AddCommand(newFindConfigCmd())

	return cmd
}
