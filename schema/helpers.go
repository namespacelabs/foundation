// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

import "sort"

const (
	GrpcProtocol = "grpc"
	HttpProtocol = "http"
)

func (sc *Schema) ExtsAndServices() []*Node {
	var nodes []*Node
	nodes = append(nodes, sc.GetExtension()...)
	nodes = append(nodes, sc.GetService()...)
	return nodes
}

func (n *Node) GetImportedPackages() []PackageName {
	return asPackages(n.GetImport()...)
}

func (n *Node) ErrorLocation() string { return n.PackageName }

func (n *Node) InitializerFor(fmwk Framework) *NodeInitializer {
	for _, i := range n.Initializers {
		if i.Framework == fmwk {
			return i
		}
	}
	return nil
}

// All frameworks that the node has codegen generated for.
// Stable order.
func (n *Node) CodegeneratedFrameworks() []Framework {
	fmwksSet := map[Framework]bool{}
	if n.ServiceFramework != Framework_FRAMEWORK_UNSPECIFIED {
		fmwksSet[n.ServiceFramework] = true
	}
	for _, i := range n.Initializers {
		fmwksSet[i.Framework] = true
	}
	for p := range n.ProvidedInFrameworks() {
		fmwksSet[p] = true
	}
	fmwks := make([]Framework, 0, len(fmwksSet))
	for f := range fmwksSet {
		fmwks = append(fmwks, f)
	}
	sort.Slice(fmwks, func(i, j int) bool {
		return fmwks[i].Number() < fmwks[j].Number()
	})

	return fmwks
}

func (n *Node) ProvidedInFrameworks() map[Framework]bool {
	fmwksSet := map[Framework]bool{}
	for _, p := range n.Provides {
		for _, a := range p.AvailableIn {
			if a.Go != nil {
				fmwksSet[Framework_GO_GRPC] = true
			}
			if a.Web != nil {
				fmwksSet[Framework_WEB] = true
			}
			if a.Nodejs != nil {
				fmwksSet[Framework_NODEJS] = true
			}
		}
	}
	return fmwksSet
}

func (s *Server) GetImportedPackages() []PackageName {
	return asPackages(s.GetImport()...)
}

func (s *Server) ErrorLocation() string { return s.PackageName }

func asPackages(strs ...string) []PackageName {
	r := make([]PackageName, len(strs))
	for k, s := range strs {
		r[k] = PackageName(s)
	}
	return r
}

func (s *Stack) GetServer(pkg PackageName) *Stack_Entry {
	if s == nil {
		return nil
	}

	for _, e := range s.Entry {
		if pkg.Equals(e.GetServer().GetPackageName()) {
			return e
		}
	}
	return nil
}

func (s *Stack) GetServerByID(id string) *Stack_Entry {
	if s == nil {
		return nil
	}

	for _, e := range s.Entry {
		if e.GetServer().GetId() == id {
			return e
		}
	}
	return nil
}

func (s *Stack) EndpointsBy(pkgName PackageName) []*Endpoint {
	var list []*Endpoint
	for _, endpoint := range s.Endpoint {
		if pkgName.Equals(endpoint.GetServerOwner()) {
			list = append(list, endpoint)
		}
	}
	return list
}

func (se *Stack_Entry) GetPackageName() PackageName {
	return PackageName(se.GetServer().GetPackageName())
}

func (se *Stack_Entry) ExtsAndServices() []*Node {
	return se.Node
}

func (se *Stack_Entry) Extensions() []*Node {
	var services []*Node
	for _, n := range se.Node {
		if n.Kind == Node_EXTENSION {
			services = append(services, n)
		}
	}
	return services
}

func (se *Stack_Entry) Services() []*Node {
	var services []*Node
	for _, n := range se.Node {
		if n.Kind == Node_SERVICE {
			services = append(services, n)
		}
	}
	return services
}

func (se *Stack_Entry) ImportsOf(pkg PackageName) []PackageName {
	for _, n := range se.ExtsAndServices() {
		if pkg.Equals(n.GetPackageName()) {
			return asPackages(n.GetImport()...)
		}
	}

	if pkg.Equals(se.Server.GetPackageName()) {
		return asPackages(se.Server.GetImport()...)
	}

	return nil
}

func (e *Endpoint) GetServerOwnerPackage() PackageName {
	return PackageName(e.ServerOwner)
}

func (e *Endpoint) HasKind(str string) bool {
	for _, md := range e.ServiceMetadata {
		if md.GetKind() == str {
			return true
		}
	}
	return false
}
