// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

//go:build darwin
// +build darwin

package disk

// We don't require filesystem information in MacOS. Filesystem information
// is used temporarily as part of #121, which doesn't affect MacOS. This
// package is expected to be removed afterwards.
func FSType(path string) (string, error) {
	return "unknown", nil
}
