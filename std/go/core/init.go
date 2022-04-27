// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	toposort "github.com/philopon/go-toposort"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

const (
	maximumInitTime = 10 * time.Millisecond
	maxStartupTime  = 2 * time.Second
)

type Reference struct {
	Package  schema.PackageName
	Typename string
}

type Package struct {
	PackageName string
}

type Provider struct {
	Package     *Package
	Typename    string
	Instantiate func(context.Context, Dependencies) (interface{}, error)
}

func (f Provider) key() string {
	if f.Typename != "" {
		return fmt.Sprintf("%s/%s", f.Package.PackageName, f.Typename)
	}
	return f.Package.PackageName
}

type result struct {
	res interface{}
	err error
}

type DependencyGraph struct {
	singletons map[string]result
	inits      []*Initializer
}

type Dependencies interface {
	Instantiate(ctx context.Context, provider Provider, f func(context.Context, interface{}) error) error
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		singletons: map[string]result{},
	}
}

func (di *DependencyGraph) Instantiate(ctx context.Context, provider Provider, f func(context.Context, interface{}) error) error {
	if singleton, ok := di.singletons[provider.key()]; ok {
		if singleton.err != nil {
			return singleton.err
		}
		return f(ctx, singleton.res)
	}

	isSingleton := provider.Typename == ""

	var path *InstantiationPath
	if !isSingleton {
		path = InstantiationPathFromContext(ctx)
	}
	childctx := path.Append(schema.PackageName(provider.Package.PackageName)).WithContext(ctx)

	start := time.Now()
	res, err := provider.Instantiate(childctx, di)
	if isSingleton {
		di.singletons[provider.key()] = result{
			res: res,
			err: err,
		}
	}
	if err != nil {
		return err
	}
	took := time.Since(start)
	if took > maximumInitTime {
		Log.Printf("[provider] %s took %d (log thresh is %d)", provider.key(), took, maximumInitTime)
	}

	return f(ctx, res)
}

type Initializer struct {
	Package *Package
	Before  []string
	After   []string
	Do      func(context.Context, Dependencies) error
}

func (di *DependencyGraph) AddInitializers(init ...*Initializer) {
	di.inits = append(di.inits, init...)
}

// Init is deprecated; use RunInitializers.
func (di *DependencyGraph) Init(ctx context.Context) error {
	return di.RunInitializers(ctx)
}

func (di *DependencyGraph) RunInitializers(ctx context.Context) error {
	resources := ServerResourcesFrom(ctx)
	if resources == nil {
		return fmt.Errorf("missing server resources")
	}

	initializationDeadline := resources.startupTime.Add(maxStartupTime)
	ctx, cancel := context.WithDeadline(ctx, initializationDeadline)
	defer cancel()

	inits, err := enforceOrder(di.inits)
	if err != nil {
		return err
	}

	for _, init := range inits {
		if *debug {
			Log.Printf("[init] initializing %s with %v deadline left", init.Package.PackageName, time.Until(initializationDeadline))
		}

		start := time.Now()
		err := init.Do(ctx, di)
		took := time.Since(start)
		if took > maximumInitTime {
			Log.Printf("[init] %s took %d (log thresh is %d)", init.Package.PackageName, took, maximumInitTime)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func enforceOrder(inits []*Initializer) ([]*Initializer, error) {
	graph := toposort.NewGraph(len(inits))

	m := map[string]struct{}{}
	for _, init := range inits {
		m[init.Package.PackageName] = struct{}{}
		for _, before := range init.Before {
			m[before] = struct{}{}
		}
		for _, after := range init.After {
			m[after] = struct{}{}
		}
	}

	for n := range m {
		graph.AddNode(n)
	}

	for _, init := range inits {
		for _, before := range init.Before {
			graph.AddEdge(init.Package.PackageName, before)
		}
		for _, after := range init.After {
			graph.AddEdge(after, init.Package.PackageName)
		}
	}

	sortedPackages, ok := graph.Toposort()
	if !ok {
		return nil, errors.New("internal failure: initializer order not fulfillable")
	}

	// XXX O(n^2)
	var sorted []*Initializer
	for _, pkg := range sortedPackages {
		for _, init := range inits {
			if init.Package.PackageName == pkg {
				sorted = append(sorted, init)
			}
		}
	}

	if len(sorted) != len(inits) {
		return nil, errors.New("internal failure: did not yield the same number of inits")
	}

	for i, init := range sorted {
		for _, before := range init.Before {
			for k := 0; k < i; k++ {
				if sorted[k].Package.PackageName == before {
					return nil, errors.New("internal failure: before: initializer order not guaranteed (verification)")
				}
			}
		}
		for _, after := range init.After {
			for k := i + 1; k < len(sorted); k++ {
				if sorted[k].Package.PackageName == after {
					return nil, errors.New("internal failure: after: initializer order not guaranteed (verification)")
				}
			}
		}
	}

	return sorted, nil
}

// MustUnwrapProto unserializes a proto from a base64 string. This is used to
// pack pre-computed protos into a binary, and is never expected to fail.
func MustUnwrapProto(b64 string, m proto.Message) proto.Message {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		panic(err)
	}
	if err := proto.Unmarshal(data, m); err != nil {
		panic(err)
	}
	return m
}
