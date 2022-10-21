// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

// Force a direct dependency so we can more easily manage fsutil's version, and
// ensure that
// https://github.com/tonistiigi/fsutil/commit/c08f2311f936733e8f998dd2395aff7605cb7379
// is in.
import _ "github.com/tonistiigi/fsutil"
