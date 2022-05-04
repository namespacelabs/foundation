// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

// Deprecated: use NewHandlers()/Handle(), etc.
func RunTool(t Tool) {
	hh := NewHandlers()
	hh.Any().HandleStack(t)
	Handle(hh)
}
