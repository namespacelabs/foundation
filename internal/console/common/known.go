// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package common

const (
	KnownStdout = "fn.console.stdout"
	KnownStderr = "fn.console.stderr"

	CatOutputTool     CatOutputType = "fn.output.tool"
	CatOutputUs       CatOutputType = "fn.output.foundation"
	CatOutputDebug    CatOutputType = "fn.output.debug"
	CatOutputWarnings CatOutputType = "fn.output.warnings"
	CatOutputErrors   CatOutputType = "fn.output.errors"
)

type CatOutputType string
