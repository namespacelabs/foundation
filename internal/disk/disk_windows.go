// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build windows
// +build windows

package disk

// We don't require filesystem information in Windows. Filesystem information
// is used temporarily as part of #121, which doesn't affect Windows. This
// package is expected to be removed afterwards.
func FSType(path string) (string, error) {
	return "unknown", nil
}
