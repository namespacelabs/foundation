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

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/framework/runtime"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/core/types"
)

func init() {
	zerolog.TimeFieldFormat = time.RFC3339Nano // Setting external package globals does not make me happy.
}

var (
	debug = flag.Bool("debug_init", false, "If set to true, emits additional initialization information.")

	datamarker atomic.Pointer[data]
)

type data struct {
	rt          *runtimepb.RuntimeConfig
	rtVcs       *runtimepb.BuildVCS
	serverName  string
	startupTime time.Time
}

var (
	// Deprecated: use ZLog.
	Log  = log.New(os.Stderr, "[ns] ", log.Ldate|log.Ltime|log.Lmicroseconds)
	ZLog = zerolog.New(os.Stderr).With().Timestamp().Str("kind", "corelog").Logger().Level(zerolog.DebugLevel)
)

func PrepareEnv(specifiedServerName string) *ServerResources {
	d, err := loadData(specifiedServerName)
	if err != nil {
		log.Fatal(err)
	}

	if !datamarker.CompareAndSwap(nil, &d) {
		log.Fatal("already initialized")
	}

	ZLog.Info().Msg("Initializing server...")

	return &ServerResources{startupTime: time.Now()}
}

func loadData(specifiedServerName string) (data, error) {
	rt, err := runtime.LoadRuntimeConfig()
	if err != nil {
		return data{}, err
	}

	rtVcs, err := runtime.LoadBuildVCS()
	if err != nil {
		return data{}, err
	}

	return data{rt, rtVcs, specifiedServerName, time.Now()}, nil
}

func initializedData() data {
	data := datamarker.Load()
	if data == nil {
		panic("not initialized")
	}
	return *data
}

func ProvideServerInfo(ctx context.Context, _ *types.ServerInfoArgs) (*types.ServerInfo, error) {
	data := initializedData()
	return &types.ServerInfo{
		ServerName: data.serverName,
		EnvName:    data.rt.Environment.Name,
		EnvPurpose: data.rt.Environment.Purpose,
		Vcs:        data.rtVcs,
	}, nil
}

func EnvIs(purpose schema.Environment_Purpose) bool {
	return initializedData().rt.Environment.Purpose == purpose.String()
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
		data := datamarker.Load()
		if data == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)

		vcsStr, _ := json.Marshal(data.rtVcs)

		fmt.Fprintf(w, "<!doctype html><html><body><pre>%s\nimage_version=%s\n%s\n%s</pre>",
			data.serverName, data.rt.Current.ImageRef, prototext.Format(data.rt.Environment), vcsStr)

		fmt.Fprintf(w, "<b>Registered endpoints</b></br><ul>")
		for _, endpoint := range registered {
			fmt.Fprintf(w, "<li><a href=%s>%s</a></li>", endpoint, endpoint)
		}
		fmt.Fprintf(w, "</ul>")

		fmt.Fprintf(w, "</body></html>")
	})
}
