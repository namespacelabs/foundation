// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provision

import (
	"namespacelabs.dev/foundation/schema"
)

type Stack struct {
	Servers   []Server
	Endpoints []*schema.Endpoint
}

func (stack *Stack) Proto() *schema.Stack {
	s := &schema.Stack{
		Endpoint: stack.Endpoints,
	}

	for _, srv := range stack.Servers {
		s.Entry = append(s.Entry, srv.StackEntry())
	}

	return s
}

func (stack *Stack) Get(pkg schema.PackageName) *Server {
	for _, s := range stack.Servers {
		if s.PackageName() == pkg {
			return &s
		}
	}
	return nil
}

func ServerPackages(stack []Server) schema.PackageList {
	var pl schema.PackageList
	for _, s := range stack {
		pl.Add(s.PackageName())
	}
	return pl
}

func (stack *Stack) ServerPackages() []schema.PackageName {
	return ServerPackages(stack.Servers).PackageNames()
}

func (stack *Stack) Contains(pkg schema.PackageName) bool { return stack.Get(pkg) != nil }
