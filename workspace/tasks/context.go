// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

type contextKey string

var (
	_sinkKey   = contextKey("fn.workspace.action.sink")
	_actionKey = contextKey("fn.workspace.action")
)