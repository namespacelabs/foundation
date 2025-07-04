// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ulimit

import "errors"

func SetFileLimit(n uint64) error {
	return errors.ErrUnsupported
}

func SetCoreFileLimit(maxCoreFileSize uint64) error {
	return errors.ErrUnsupported
}
