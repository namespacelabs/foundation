// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package core

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/framework/runtime"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/core/types"
)

var (
	debug = flag.Bool("debug_init", false, "If set to true, emits additional initialization information.")

	rt          *runtimepb.RuntimeConfig
	rtVcs       *runtimepb.BuildVCS
	serverName  string
	initialized uint32
)

var Log = log.New(os.Stderr, "[ns] ", log.Ldate|log.Ltime|log.Lmicroseconds)

func PrepareEnv(specifiedServerName string) *ServerResources {
	if !atomic.CompareAndSwapUint32(&initialized, 0, 1) {
		Log.Fatal("already initialized")
	}

	Log.Println("Initializing server...")

	var err error
	rt, err = runtime.LoadRuntimeConfig()
	if err != nil {
		Log.Fatal(err)
	}

	rtVcs, err = runtime.LoadBuildVCS()
	if err != nil {
		Log.Fatal(err)
	}

	serverName = specifiedServerName

	return &ServerResources{startupTime: time.Now()}
}

func ProvideServerInfo(ctx context.Context, _ *types.ServerInfoArgs) (*types.ServerInfo, error) {
	return &types.ServerInfo{
		ServerName: serverName,
		EnvName:    rt.Environment.Name,
		EnvPurpose: rt.Environment.Purpose,
		Vcs:        rtVcs,
	}, nil
}

func EnvIs(purpose schema.Environment_Purpose) bool {
	return rt.Environment.Purpose == purpose.String()
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

		vcsStr, _ := json.Marshal(rtVcs)

		fmt.Fprintf(w, "<!doctype html><html><body><pre>%s\nimage_version=%s\n%s\n%s</pre>",
			serverName, rt.Current.ImageRef, prototext.Format(rt.Environment), vcsStr)

		fmt.Fprintf(w, "<b>Registered endpoints</b></br><ul>")
		for _, endpoint := range registered {
			fmt.Fprintf(w, "<li><a href=%s>%s</a></li>", endpoint, endpoint)
		}
		fmt.Fprintf(w, "</ul>")

		fmt.Fprintf(w, "</body></html>")
	})
}
