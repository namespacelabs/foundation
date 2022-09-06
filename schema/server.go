// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

// Returns true if a server should be considered for location-less invocations (e.g. `ns deploy`).
func (s *Server) RunByDefault() bool {
	if s == nil || s.GetTestonly() || s.GetClusterAdmin() {
		return false
	}

	return true
}
