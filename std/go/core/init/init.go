// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package init

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/protobuf/proto"
)

const maximumInitTime = 10 * time.Millisecond

var Log = log.New(os.Stderr, "[foundation] ", log.Ldate|log.Ltime|log.Lmicroseconds)

type Factory struct {
	PackageName string
	Typename    string
	Do          func(context.Context, *CallerFactory) (interface{}, error)
}

func (f *Factory) Desc() string {
	if f.Typename != "" {
		return fmt.Sprintf("%s/%s", f.PackageName, f.Typename)
	}
	return f.PackageName
}

type key struct {
	PackageName string
	Typename    string
}

type result struct {
	res interface{}
	err error
}

type depInitializer struct {
	factories map[key]*Factory
	cache     map[key]*result
	inits     []*Initializer
}

func MakeInitializer() *depInitializer {
	return &depInitializer{
		factories: map[key]*Factory{},
		cache:     map[key]*result{},
	}
}

func (di *depInitializer) Add(f Factory) {
	di.factories[key{PackageName: f.PackageName, Typename: f.Typename}] = &f
}

func (di *depInitializer) Get(ctx context.Context, caller Caller, pkg string, typ string) (interface{}, error) {
	k := key{PackageName: pkg, Typename: typ}

	f, ok := di.factories[k]
	if !ok {
		return nil, fmt.Errorf("No factory found for type %s in package %s.", typ, pkg)
	}

	cf := caller.append(pkg)

	start := time.Now()
	res, err := f.Do(ctx, cf)
	took := time.Since(start)
	if took > maximumInitTime {
		Log.Printf("[factory] %s took %d (log thresh is %d)", f.Desc(), took, maximumInitTime)
	}

	return res, err
}

func (di *depInitializer) GetSingleton(ctx context.Context, pkg string, typ string) (interface{}, error) {
	k := key{PackageName: pkg, Typename: typ}
	if res, ok := di.cache[k]; ok {
		return res.res, res.err
	}

	res, err := di.Get(ctx, Caller{}, pkg, typ)
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
