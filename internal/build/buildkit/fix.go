// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

// Force a direct dependency so we can more easily manage fsutil's version, and
// ensure that
// https://github.com/tonistiigi/fsutil/commit/c08f2311f936733e8f998dd2395aff7605cb7379
// is in.
import _ "github.com/tonistiigi/fsutil"
