// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"fmt"
	"strings"
)

func (d *Diagnostics) Failed() bool {
	return d.Terminated && d.ExitCode > 0
}

func (cw *ContainerWaitStatus) WaitStatus() string {
	var inits []string
	for _, init := range cw.Initializers {
		inits = append(inits, fmt.Sprintf("%s: %s", init.Name, init.StatusLabel))
	}

	joinedInits := strings.Join(inits, "; ")

	switch len(cw.Containers) {
	case 0:
		return joinedInits
	case 1:
		return box(cw.Containers[0].StatusLabel, joinedInits)
	default:
		var labels []string
		for _, ctr := range cw.Containers {
			labels = append(labels, fmt.Sprintf("%s: %s", ctr.Name, ctr.StatusLabel))
		}

		return box(fmt.Sprintf("{%s}", strings.Join(labels, "; ")), joinedInits)
	}
}

func box(a, b string) string {
	if b == "" {
		return a
	}

	return fmt.Sprintf("%s [%s]", a, b)
}
