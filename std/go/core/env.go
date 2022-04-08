// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"sync/atomic"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/schema"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
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

func PrepareEnv(specifiedServerName string) *ServerResources {
	if !atomic.CompareAndSwapUint32(&initialized, 0, 1) {
		fninit.Log.Fatal("already initialized")
	}

	fninit.Log.Println("Initializing server...")

	env = &schema.Environment{}
	if err := protojson.Unmarshal([]byte(*serializedEnv), env); err != nil {
		fninit.Log.Fatal("failed to parse environment", err)
	}

	serverName = specifiedServerName

	return &ServerResources{startupTime: time.Now()}
}

func EnvIs(purpose schema.Environment_Purpose) bool {
	return env.Purpose == purpose
}

type frameworkKey string

const ctxResourcesKey = frameworkKey("ns.serverresources")

func WithResources(ctx context.Context, res *ServerResources) context.Context {
	return context.WithValue(ctx, ctxResourcesKey, res)
}

func ServerResourcesFrom(ctx context.Context) *ServerResources {
	v := ctx.Value(ctxResourcesKey)
