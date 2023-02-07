// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package filewatch

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/filewatcher"
)

func WithFileWatch(rootCmd *cobra.Command) {
	preRun := rootCmd.PersistentPreRunE

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		filewatcher.SetupFileWatcher()

		if preRun != nil {
			return preRun(cmd, args)
		}

		return nil
	}

	rootCmd.PersistentFlags().BoolVar(&filewatcher.FileWatcherUsePolling, "filewatcher_use_polling",
		filewatcher.FileWatcherUsePolling, "If set to true, uses polling to observe file system events.")
	_ = rootCmd.PersistentFlags().MarkHidden("filewatcher_use_polling")
}
