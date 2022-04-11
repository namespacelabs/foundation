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

const maximumInitTime = 10 * time.Millisecond

type Provider struct {
	PackageName schema.PackageName
	Typename    string
	Do          func(context.Context, schema.PackageName) (interface{}, error)
}

func (f *Provider) Desc() string {
	if f.Typename != "" {
		return fmt.Sprintf("%s/%s", f.PackageName, f.Typename)
	}
	return f.PackageName.String()
}

type key struct {
	PackageName schema.PackageName
	Typename    string
}

type result struct {
	res interface{}
	err error
}

type depInitializer struct {
	providers map[key]*Provider
	cache     map[key]*result
	inits     []*Initializer
}

func MakeInitializer() *depInitializer {
	return &depInitializer{
		providers: map[key]*Provider{},
		cache:     map[key]*result{},
	}
}

func (di *depInitializer) Add(p Provider) {
	di.providers[key{PackageName: p.PackageName, Typename: p.Typename}] = &p
}

func (di *depInitializer) Get(ctx context.Context, pkg schema.PackageName, typ string) (interface{}, error) {
	k := key{PackageName: pkg, Typename: typ}

	p, ok := di.providers[k]
	if !ok {
		return nil, fmt.Errorf("No provider found for type %s in package %s.", typ, pkg)
	}

	start := time.Now()
	res, err := p.Do(ctx, pkg)
	took := time.Since(start)
	if took > maximumInitTime {
		Log.Printf("[provider] %s took %d (log thresh is %d)", p.Desc(), took, maximumInitTime)
	}

	return res, err
}

func (di *depInitializer) GetSingleton(ctx context.Context, pkg schema.PackageName, typ string) (interface{}, error) {
	k := key{PackageName: pkg, Typename: typ}
	if res, ok := di.cache[k]; ok {
		return res.res, res.err
	}

	res, err := di.Get(resetInstantiationPath(ctx), pkg, typ)
	di.cache[k] = &result{res: res, err: err}
	return res, err
}

type Initializer struct {
	PackageName string
	Do          func(context.Context) error
}

func (di *depInitializer) AddInitializer(init Initializer) {
	di.inits = append(di.inits, &init)
}

func (di *depInitializer) Init(ctx context.Context) error {
	for _, init := range di.inits {
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
