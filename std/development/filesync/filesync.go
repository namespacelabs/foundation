// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package filesync

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/wscontents"
)

func StartFileSyncServer(configuration *FileSyncConfiguration) {
	if configuration.RootDir == "" {
		log.Fatal("root_dir is missing")
	}

	grpcServer := grpc.NewServer()

	wsremote.RegisterFileSyncServiceServer(grpcServer, sinkServer{configuration})

	filesyncHost := fmt.Sprintf("0.0.0.0:%d", configuration.Port)
	lis, err := net.Listen("tcp", filesyncHost)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("FileSync listening on %s. Root dir: \"%s\"", filesyncHost, configuration.RootDir)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

type sinkServer struct {
	configuration *FileSyncConfiguration
}

func (s sinkServer) Push(ctx context.Context, req *wsremote.PushRequest) (*wsremote.PushResponse, error) {
	pkg := filepath.Join(req.GetSignature().GetModuleName(), req.GetSignature().GetRel())

	basePath := filepath.Join(s.configuration.RootDir, pkg)

	for k, ev := range req.FileEvent {
		if ev.Path == "" {
			return nil, status.Errorf(codes.InvalidArgument, "file_event[%d]: no path", k)
		}

		filePath := filepath.Join(basePath, ev.Path)

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

	return &wsremote.PushResponse{}, nil
}
