// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/reverseproxy"
	"namespacelabs.dev/foundation/std/dev/controller/admin"
	"namespacelabs.dev/foundation/std/development/filesync"
)

var (
	serializedConf = flag.String("configuration", "", "Serialized configuration in JSON.")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()

	configuration := &admin.Configuration{}
	if err := protojson.Unmarshal([]byte(*serializedConf), configuration); err != nil {
		log.Fatal(err)
	}

	if configuration.PackageBase == "" {
		log.Fatal("package_base is missing")
	}

	for _, backend := range configuration.Backend {
		if len(backend.GetExecution().GetArgs()) == 0 {
			log.Fatal("backend.execution.args can't be empty")
		}
	}

	ctx := context.Background()
	running := &running{configuration: configuration, processes: map[int]runningProcess{}}
	r := mux.NewRouter()

	for index, backend := range configuration.Backend {
		running.restartBackend(index)

		if httpPass := backend.HttpPass; httpPass != nil {
			if configuration.RevproxyPort == 0 {
				log.Fatal("http_pass without revproxy_port configuration")
			}

			r.PathPrefix(httpPass.UrlPrefix).
				Handler(reverseproxy.Make(httpPass.Backend, reverseproxy.DefaultLocalProxy()))
		}
	}

	if configuration.FilesyncPort > 0 {
		go func() {
			filesync.StartFileSyncServer(&filesync.FileSyncConfiguration{
				RootDir: configuration.PackageBase,
				Port:    configuration.FilesyncPort,
			})
		}()
	}

	if configuration.RevproxyPort > 0 {
		httpHost := fmt.Sprintf("0.0.0.0:%d", configuration.RevproxyPort)
		srv := &http.Server{
			Handler:      r,
			Addr:         httpHost,
			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
			BaseContext:  func(l net.Listener) context.Context { return ctx },
		}

		log.Printf("ReverseProxy listening on %s", httpHost)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	} else {
		select {}
	}
}

func makeExecution(ctx context.Context, configuration *admin.Configuration, packageName string, execution *admin.Execution) *exec.Cmd {
	cmd := exec.CommandContext(ctx, execution.Args[0], execution.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Join(configuration.PackageBase, packageName)
	cmd.Env = append(os.Environ(), execution.AdditionalEnv...)
	return cmd
}

type running struct {
	configuration *admin.Configuration
	mu            sync.Mutex
	processes     map[int]runningProcess
}

type runningProcess struct {
	cmd        *exec.Cmd
	cancel     func()
	doneSignal chan struct{}
}

func (r *running) restartBackend(index int) {
	r.mu.Lock()
	existing, ok := r.processes[index]
	r.mu.Unlock()

	if ok {
		existing.cancel()
		for range existing.doneSignal {
			// Wait until Wait() returns.
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.processes[index]; ok {
		// Someone beat us and already started it.
		return
	}

	backend := r.configuration.Backend[index]
	ctx, done := context.WithCancel(context.Background())
	cmd := makeExecution(ctx, r.configuration, backend.PackageName, backend.Execution)
	if err := cmd.Start(); err != nil {
		log.Fatal("cmd.Start failed", index, err)
	}

	log.Printf("Started backend index %d.", index)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		if err := cmd.Wait(); err != nil {
			log.Fatal("cmd.Wait failed", index, err)
		}
	}()

	r.processes[index] = runningProcess{
		cmd:        cmd,
		cancel:     done,
		doneSignal: ch,
	}
}
