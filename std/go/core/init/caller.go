// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package init

import (
	"fmt"
	"strings"
)

type instantiation struct {
	PackageName string
	Instance    string
}

type Caller struct {
	path []instantiation
}

type CallerFactory struct {
	caller      *Caller
	packageName string
}

func (c *Caller) append(pkg string) *CallerFactory {
	return &CallerFactory{
		caller:      c,
		packageName: pkg,
	}
}

func (c *Caller) LastPkg() string {
	if len(c.path) == 0 {
		return ""
	}
	return c.path[len(c.path)-1].PackageName
}

func (c *Caller) String() string {
	var inst []string
	for _, step := range c.path {
		inst = append(inst, fmt.Sprintf("%s:%s", step.PackageName, step.Instance))
	}
	return strings.Join(inst, "->")
}

func (cf *CallerFactory) MakeCaller(inst string) Caller {
	copy := *cf.caller
	copy.path = append(copy.path, instantiation{
		PackageName: cf.packageName,
		Instance:    inst,
	})
	return copy
}
