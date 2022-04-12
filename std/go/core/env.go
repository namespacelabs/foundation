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

type DepInitializer struct {
	inits []Initializer
}

type Initializer struct {
	PackageName string
	Instance    string
	DependsOn   []string
	Do          func(context.Context) error
}

func (init Initializer) Desc() string {
	if init.Instance != "" {
		return fmt.Sprintf("%s/%s", init.PackageName, init.Instance)
	}
	return init.PackageName
}

func (di *DepInitializer) Register(init Initializer) {
	di.inits = append(di.inits, init)
}

func (di *DepInitializer) Wait(ctx context.Context) error {
	resources := ServerResourcesFrom(ctx)
	if resources == nil {
		return fmt.Errorf("missing server resources")
	}

	initializationDeadline := resources.startupTime.Add(maxStartupTime)
	ctx, cancel := context.WithDeadline(ctx, initializationDeadline)
	defer cancel()

	Log.Printf("[init] starting with %v initialization deadline left", time.Until(initializationDeadline))

	for k, init := range di.inits {
		// XXX at the moment we don't make sure of dependency information, but we can
		// to enable concurrent initialization.

		if *debug {
			Log.Printf("[init] initializing %s/%s with %v deadline left", init.PackageName, init.Instance, time.Until(initializationDeadline))
		}

		start := time.Now()
		err := init.Do(ctx)
		took := time.Since(start)

		if took > maximumInitTime {
			Log.Printf("[init] %s took %d (log thresh is %d)", init.Desc(), took, maximumInitTime)
		}

		if err != nil {
			Log.Printf("Failed to initialize %q: %v.", di.inits[k].Desc(), err)
			for j := k + 1; j < len(di.inits); j++ {
				// If one of the dependencies already failed, we can't assume there's a clean state to follow
				// up with. And thus we bail out.
				Log.Printf("Not initializing %q, due to previous failure.", di.inits[j].Desc())
			}

			return fmt.Errorf("initialization failed: %w", err)
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
	if v == nil {
		return nil
	}

	return v.(*ServerResources)
}

func StatusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "<!doctype html><html><body><pre>%s image_version=%s\n%s</pre></body></html>",
			serverName, *imageVer, prototext.Format(env))
	})
}
