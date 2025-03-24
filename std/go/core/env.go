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
	"runtime"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	fnruntime "namespacelabs.dev/foundation/framework/runtime"
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
	rt                *runtimepb.RuntimeConfig
	rtVcs             *runtimepb.BuildVCS
	rtSecretChecksums []*runtimepb.SecretChecksum

	serverName  string
	startupTime time.Time
}

var (
	// Deprecated: use ZLog.
	Log  = log.New(os.Stderr, "[ns] ", log.Ldate|log.Ltime|log.Lmicroseconds)
	ZLog = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.DebugLevel)
)

func PrepareEnv(specifiedServerName string) (*ServerResources, string) {
	d, err := loadData(specifiedServerName)
	if err != nil {
		log.Fatal(err)
	}

	rev := d.rtVcs.GetRevision()

	if !datamarker.CompareAndSwap(nil, &d) {
		log.Fatal("already initialized")
	}

	ZLog.Info().Msg("Initializing server...")

	return &ServerResources{startupTime: time.Now()}, rev
}

func loadData(specifiedServerName string) (data, error) {
	rt, err := fnruntime.LoadRuntimeConfig()
	if err != nil {
		return data{}, err
	}

	rtVcs, err := fnruntime.LoadBuildVCS()
	if err != nil {
		return data{}, err
	}

	secChecksums, err := fnruntime.LoadSecretChecksums()
	if err != nil {
		return data{}, err
	}

	return data{rt, rtVcs, secChecksums, specifiedServerName, time.Now()}, nil
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
		ServerName:     data.serverName,
		EnvName:        data.rt.Environment.Name,
		EnvPurpose:     data.rt.Environment.Purpose,
		Vcs:            data.rtVcs,
		SecretChecksum: data.rtSecretChecksums,
	}, nil
}

func EnvPurpose() schema.Environment_Purpose {
	v, ok := schema.Environment_Purpose_value[initializedData().rt.Environment.Purpose]
	if ok {
		return schema.Environment_Purpose(v)
	}
	return schema.Environment_PURPOSE_UNKNOWN
}

func EnvIs(purpose schema.Environment_Purpose) bool {
	return EnvPurpose() == purpose
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

		vals, _ := json.MarshalIndent(struct {
			VCS            any    `json:"vcs"`
			ImageVersion   string `json:"image_version"`
			RuntimeVersion string `json:"runtime_version"`
			GOOS           string `json:"GOOS"`
			GOARCH         string `json:"GOARCH"`
			Environment    any    `json:"env"`
		}{
			VCS:            data.rtVcs,
			ImageVersion:   data.rt.Current.ImageRef,
			RuntimeVersion: runtime.Version(),
			GOOS:           runtime.GOOS,
			GOARCH:         runtime.GOARCH,
			Environment:    data.rt.Environment,
		}, "", "  ")

		fmt.Fprintf(w, "<!doctype html><html><body><pre>%s\n\n%s\n</pre>", data.serverName, vals)

		fmt.Fprintf(w, "<b>Registered endpoints</b></br><ul>")
		for _, endpoint := range registered {
			fmt.Fprintf(w, "<li><a href=%s>%s</a></li>", endpoint, endpoint)
		}
		fmt.Fprintf(w, "</ul>")

		fmt.Fprintf(w, "</body></html>")
	})
}
