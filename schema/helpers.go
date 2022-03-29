// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

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