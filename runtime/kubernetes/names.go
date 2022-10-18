// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"regexp"
	"strings"
)

var bad = regexp.MustCompile("[^_0-9a-zA-Z]")

// It's fairly normal that Kubernetes only accepts keys which match the
// [-._a-zA-Z0-9]+ regex. The strategy here is to replace all non-letter
// non-number characters with "-", and then join each segment with ".".
//
// Example: foobar.com/bar:key becomes foobar-com-bar.key
func cleanName(parts ...string) string {
	clean := make([]string, len(parts))
	for k, str := range parts {
		clean[k] = cleanOnePart(str)
	}
	return strings.Join(clean, ".")
}

func cleanOnePart(str string) string {
	return bad.ReplaceAllLiteralString(str, "-")
}
