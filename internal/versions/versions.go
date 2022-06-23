// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package versions

// APIVersion represents the overall version of Namespaces's semantics, which
// are built into Namespace itself (i.e. is not versioned as part of the
// foundation repository). Whenever new non-backwards compatible semantics are
// added to Namespace, this number must be bumped.
const APIVersion = 36

const IntroducedGrpcTranscodeNode = 35

// MinimumAPIVersion represents the minimum requested version that this version
// of foundation supports. If a module requests, e.g. a minimum version of 28,
// which is below the version here specified, then Namespace will fail with a
// error that says our version of Namespace is too recent. This is used during
// development when maintaining backwards compatibility is too expensive.ß
const MinimumAPIVersion = 29

// Embedded into provisioning tools.
const ToolAPIVersion = 2

// Allow for global cache invalidation.
const CacheVersion = 1
