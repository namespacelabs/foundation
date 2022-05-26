// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tui

import "github.com/charmbracelet/lipgloss"

var (
	listMainStyle = lipgloss.NewStyle().Margin(0, 2, 1)
	askMainStyle  = lipgloss.NewStyle().Margin(0, 2, 1, 4) // A wider margin is used to align with lists.
	titleStyle    = lipgloss.NewStyle().Bold(true)
	descStyle     = lipgloss.NewStyle()
)
