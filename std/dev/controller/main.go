// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/reverseproxy"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/std/dev/controller/admin"
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

		for _, hook := range backend.OnChange {
			if len(hook.GetExecution().GetArgs()) == 0 {
				log.Fatal("backend.on_change.execution.args can't be empty")
			}
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
			grpcServer := grpc.NewServer()

			wsremote.RegisterFileSyncServiceServer(grpcServer, sinkServer{configuration, running})

			filesyncHost := fmt.Sprintf("0.0.0.0:%d", configuration.FilesyncPort)
			lis, err := net.Listen("tcp", filesyncHost)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("FileSync listening on %s", filesyncHost)
			if err := grpcServer.Serve(lis); err != nil {
				log.Fatal(err)
			}
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

func spawnExecution(ctx context.Context, configuration *admin.Configuration, packageName string, execution *admin.Execution) error {
	return makeExecution(ctx, configuration, packageName, execution).Run()
}

func makeExecution(ctx context.Context, configuration *admin.Configuration, packageName string, execution *admin.Execution) *exec.Cmd {
	cmd := exec.CommandContext(ctx, execution.Args[0], execution.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Join(configuration.PackageBase, packageName)
	cmd.Env = append(os.Environ(), execution.AdditionalEnv...)
	return cmd
}

type sinkServer struct {
	configuration *admin.Configuration
	running       *running
}

func (s sinkServer) Push(ctx context.Context, req *wsremote.PushRequest) (*wsremote.PushResponse, error) {
	pkg := filepath.Join(req.GetSignature().GetModuleName(), req.GetSignature().GetRel())

	for index, backend := range s.configuration.Backend {
		if backend.PackageName == pkg {
			basePath := filepath.Join(s.configuration.PackageBase, pkg)

			hookMap := map[int]struct{}{}

			for k, ev := range req.FileEvent {
				if ev.Path == "" {
					return nil, status.Errorf(codes.InvalidArgument, "file_event[%d]: no path", k)
				}

				filePath := filepath.Join(basePath, ev.Path)

				for k, hook := range backend.OnChange {
					if slices.Contains(hook.Path, ev.Path) {
						hookMap[k] = struct{}{} // Mark this hook for execution.
					}
				}

				switch ev.Event {
				case wscontents.FileEvent_WRITE:
					if err := ioutil.WriteFile(filePath, ev.NewContents, fs.FileMode(ev.Mode)); err != nil {
						return nil, err
					}

				case wscontents.FileEvent_REMOVE:
					if err := os.Remove(filePath); err != nil {
						return nil, err
					}

				case wscontents.FileEvent_MKDIR:
					if err := os.Mkdir(filePath, fs.FileMode(ev.Mode)); err != nil {
						return nil, err
					}

				default:
					return nil, status.Errorf(codes.InvalidArgument, "file_event[%d]: unrecognized event type: %s", k, ev.Event)
				}
			}

			// Follow the hook definition order.
			restart := false
			for k, hook := range backend.OnChange {
				if _, ok := hookMap[k]; !ok {
					continue
				}

				if err := spawnExecution(ctx, s.configuration, backend.PackageName, hook.Execution); err != nil {
					return nil, err
				}

				if hook.RestartAfterExecution {
					restart = true
				}
			}

			if restart {
				s.running.restartBackend(index)
			}

			return &wsremote.PushResponse{}, nil
		}
	}

	return nil, status.Errorf(codes.InvalidArgument, "package not configured: %s", pkg)
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
