// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/schema"
)

var (
	serializedEnv = flag.String("env_json", "", "The environment definition, serialized as JSON.")
	imageVer      = flag.String("image_version", "", "The version being run.")
	debug         = flag.Bool("debug_init", false, "If set to true, emits additional initialization information.")

	env         *schema.Environment
	serverName  string
	initialized uint32
)

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

func ProvideServerInfo(ctx context.Context, _ *ServerInfoArgs) (*ServerInfo, error) {
	return &ServerInfo{
		ServerName: serverName,
		EnvName:    env.Name,
	}, nil
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
	if v == nil {
		return nil
	}

	return v.(*ServerResources)
}

func StatusHandler(registered []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "<!doctype html><html><body><pre>%s\nimage_version=%s\n%s</pre>",
			serverName, *imageVer, prototext.Format(env))

		fmt.Fprintf(w, "<b>Registered endpoints</b></br><ul>")
		for _, endpoint := range registered {
			fmt.Fprintf(w, "<li><a href=%s>%s</a></li>", endpoint, endpoint)
		}
		fmt.Fprintf(w, "</ul>")

		fmt.Fprintf(w, "</body></html>")
	})
}
