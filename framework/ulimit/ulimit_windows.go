// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build windows
// +build windows

package ulimit

func SetFileLimit(n uint64) error {
	// noop, on windows there is no open file limit.
	return nil
}
