// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package inputs

import "github.com/charmbracelet/bubbles/textinput"

func NewTextInput() textinput.Model {
	input := textinput.New()

	input.CursorStyle = cursorStyle
	input.PromptStyle = focusedStyle
	input.TextStyle = focusedStyle

	return input
}
