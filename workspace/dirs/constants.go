// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package dirs

var DirsToAvoid = []string{"node_modules"}
var AllDirsToAvoid = append([]string{".git", ".parcel-cache"}, DirsToAvoid...)
