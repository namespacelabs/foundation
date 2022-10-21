// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/core/types"
)

var (
	debug = flag.Bool("debug_init", false, "If set to true, emits additional initialization information.")

	rt          *runtime.RuntimeConfig
	rtVcs       *runtime.BuildVCS
	serverName  string
	initialized uint32
)

var Log = log.New(os.Stderr, "[ns] ", log.Ldate|log.Ltime|log.Lmicroseconds)

func LoadRuntimeConfig() (*runtime.RuntimeConfig, error) {
	configBytes, err := os.ReadFile("/namespace/config/runtime.json")
	if err != nil {
		return nil, rpcerrors.Errorf(codes.Internal, "failed to unwrap runtime configuration: %w", err)
	}

	rt := &runtime.RuntimeConfig{}
	if err := json.Unmarshal(configBytes, rt); err != nil {
		return nil, rpcerrors.Errorf(codes.Internal, "failed to unmarshal runtime configuration: %w", err)
	}

	return rt, nil
}

func LoadResources() (*resources.Parsed, error) {
	configBytes, err := os.ReadFile("/namespace/config/resources.json")
	if err != nil {
		return nil, rpcerrors.Errorf(codes.Internal, "failed to unwrap resource configuration: %w", err)
	}

	return resources.ParseResourceData(configBytes)
}

func PrepareEnv(specifiedServerName string) *ServerResources {
	if !atomic.CompareAndSwapUint32(&initialized, 0, 1) {
		Log.Fatal("already initialized")
	}

	Log.Println("Initializing server...")

	var err error
	rt, err = LoadRuntimeConfig()
	if err != nil {
		Log.Fatal(err)
	}

	serializedVCS, err := os.ReadFile("/namespace/config/buildvcs.json")
	if err != nil {
		if !os.IsNotExist(err) {
			Log.Fatal("failed to load VCS information", err)
		}
	} else {
		vcs := &runtime.BuildVCS{}
		if err := json.Unmarshal(serializedVCS, vcs); err != nil {
			Log.Fatal("failed to parse VCS information", err)
		}
		rtVcs = vcs
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
