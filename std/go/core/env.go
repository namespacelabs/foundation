// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

var (
	serializedEnv = flag.String("env_json", "", "The environment definition, serialized as JSON.")
	imageVer      = flag.String("image_version", "", "The version being run.")
	debug         = flag.Bool("debug_init", false, "If set to true, emits additional initialization information.")

	maxStartupTime = 2 * time.Second

	env         *schema.Environment
	serverName  string
	initialized uint32
)

const maximumInitTime = 10 * time.Millisecond

var Log = log.New(os.Stderr, "[foundation] ", log.Ldate|log.Ltime|log.Lmicroseconds)

func PrepareEnv(specifiedServerName string) *ServerResources {
	if !atomic.CompareAndSwapUint32(&initialized, 0, 1) {
		Log.Fatal("already initialized")
	}

	Log.Println("Initializing server...")

	env = &schema.Environment{}
	if err := protojson.Unmarshal([]byte(*serializedEnv), env); err != nil {
		Log.Fatal("failed to parse environment", err)
	}

	serverName = specifiedServerName

	return &ServerResources{startupTime: time.Now()}
}

func EnvIs(purpose schema.Environment_Purpose) bool {
	return env.Purpose == purpose
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

type key struct {
	PackageName string
	Instance    string
}

type result struct {
	res interface{}
	err error
}

type DepInitializer struct {
	factories map[key]*Factory
	cache     map[key]*result
	inits     []*Initializer
}

type Factory struct {
	PackageName string
	Instance    string
	Singleton   bool
	Do          func(context.Context) (interface{}, error)
}

func (f Factory) Desc() string {
	if f.Instance != "" {
		return fmt.Sprintf("%s/%s", f.PackageName, f.Instance)
	}
	return f.PackageName
}

func (di *DepInitializer) Add(f Factory) {
	if di.factories == nil {
		di.factories = make(map[key]*Factory)
	}
	di.factories[key{PackageName: f.PackageName, Instance: f.Instance}] = &f
}

func (di *DepInitializer) Get(ctx context.Context, pkg string, inst string) (interface{}, error) {
	k := key{PackageName: pkg, Instance: inst}
	if res, ok := di.cache[k]; ok {
		return res.res, res.err
	}

	f, ok := di.factories[k]
	if !ok {
		return nil, fmt.Errorf("No factory found found for instance %s in package %s.", inst, pkg)
	}

	start := time.Now()
	res, err := f.Do(ctx)
	took := time.Since(start)
	if took > maximumInitTime {
		Log.Printf("[factory] %s took %d (log thresh is %d)", f.Desc(), took, maximumInitTime)
	}

	if f.Singleton {
		di.cache[k] = &result{res: res, err: err}
	}
	return res, err
}

type Initializer struct {
	PackageName string
	Do          func(context.Context) error
}

func (di *DepInitializer) Register(init Initializer) {
	di.inits = append(di.inits, &init)
}

func (di *DepInitializer) Init(ctx context.Context) error {
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

type frameworkKey string

const ctxResourcesKey = frameworkKey("ns.serverresources")

func WithResources(ctx context.Context, res *ServerResources) context.Context {
	return context.WithValue(ctx, ctxResourcesKey, res)
}

func ServerResourcesFrom(ctx context.Context) *ServerResources {
	v := ctx.Value(ctxResourcesKey)
