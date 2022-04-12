// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

const (
	maximumInitTime = 10 * time.Millisecond
	maxStartupTime  = 2 * time.Second
)

type Reference struct {
	Package schema.PackageName
	Scope   string
}

type Provider struct {
	Package schema.PackageName
	Scope   string
	Do      func(context.Context) (interface{}, error)
}

func (f *Provider) Desc() string {
	if f.Scope != "" {
		return fmt.Sprintf("%s/%s", f.Package, f.Scope)
	}
	return f.Package.String()
}

type result struct {
	res interface{}
	err error
}

type depInitializer struct {
	providers  map[Reference]Provider
	singletons map[Reference]result
	inits      []Initializer
}

func MakeInitializer() *depInitializer {
	return &depInitializer{
		providers:  map[Reference]Provider{},
		singletons: map[Reference]result{},
	}
}

func (di *depInitializer) Add(p Provider) {
	di.providers[Reference{Package: p.Package, Scope: p.Scope}] = p
}

func (di *depInitializer) Instantiate(ctx context.Context, ref Reference, f func(context.Context, interface{}) error) error {
	if singleton, ok := di.singletons[ref]; ok {
		if singleton.err != nil {
			return singleton.err
		}
		return f(ctx, singleton.res)
	}

	p, ok := di.providers[ref]
	if !ok {
		return fmt.Errorf("No provider found for type %s in package %s.", ref.Scope, ref.Package)
	}

	isSingleton := ref.Scope == ""
	var path *InstantiationPath
	if !isSingleton {
		path = PathFromContext(ctx)
	}
	childctx := path.Append(ref.Package).WithContext(ctx)

	start := time.Now()
	res, err := p.Do(childctx)
	if isSingleton {
		di.singletons[ref] = result{
			res: res,
			err: err,
		}
	}
	if err != nil {
		return err
	}
	took := time.Since(start)
	if took > maximumInitTime {
		Log.Printf("[provider] %s took %d (log thresh is %d)", p.Desc(), took, maximumInitTime)
	}

	return f(ctx, res)
}

type Initializer struct {
	PackageName string
	Do          func(context.Context) error
}

func (di *depInitializer) AddInitializer(init Initializer) {
	di.inits = append(di.inits, init)
}

func (di *depInitializer) Init(ctx context.Context) error {
	resources := ServerResourcesFrom(ctx)
	if resources == nil {
		return fmt.Errorf("missing server resources")
	}

	initializationDeadline := resources.startupTime.Add(maxStartupTime)
	ctx, cancel := context.WithDeadline(ctx, initializationDeadline)
	defer cancel()

	for _, init := range di.inits {
		if *debug {
			Log.Printf("[init] initializing %s with %v deadline left", init.PackageName, time.Until(initializationDeadline))
		}

		start := time.Now()
		err := init.Do(ctx)
		took := time.Since(start)
		if took > maximumInitTime {
			Log.Printf("[init] %s took %d (log thresh is %d)", init.PackageName, took, maximumInitTime)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// MustUnwrapProto unserializes a proto from a base64 string. This is used to
// pack pre-computed protos into a binary, and is never expected to fail.
func MustUnwrapProto(b64 string, m proto.Message) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		panic(err)
	}
	if err := proto.Unmarshal(data, m); err != nil {
		panic(err)
	}
}
