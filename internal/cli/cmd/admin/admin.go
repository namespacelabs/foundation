// Copyright 2024 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package admin

import (
	"github.com/spf13/cobra"
)

func NewAdminCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "admin",
		Short:  "Partner administration commands.",
		Hidden: hidden,
	}

	cmd.AddCommand(newSignPartnerTokenCmd())

	return cmd
}
