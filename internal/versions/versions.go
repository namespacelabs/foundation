// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package versions

// APIVersion represents the overall version of Foundation's semantics, which
// are built into Foundation itself (i.e. is not versioned as part of the
// foundation repository). Whenever new non-backwards compatible semantics are
// added to Foundation, this number must be bumped.
const APIVersion = 28

// Embedded into provisioning tools.
const ToolAPIVersion = 2

// Allow for global cache invalidation.
const CacheVersion = 1
