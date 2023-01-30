// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/internal/providers/aws"
	"namespacelabs.dev/foundation/internal/providers/gcp"
	"namespacelabs.dev/foundation/internal/providers/gcp/gke"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/protolog"
	awsconf "namespacelabs.dev/foundation/universe/aws/configuration"
)

const (
	portName    = "server-port"
	connTimeout = time.Minute // TODO reduce - we've seen slow connections in CI
)

type RemoteOrchestrator struct {
	cluster runtime.ClusterNamespace
	server  runtime.Deployable
}

func (c *RemoteOrchestrator) Connect(ctx context.Context) (*grpc.ClientConn, error) {
	orch := compute.On(ctx)
	sink := tasks.SinkFrom(ctx)

	return grpc.DialContext(ctx, "orchestrator",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			patchedContext := compute.AttachOrch(tasks.WithSink(ctx, sink), orch)

			conn, err := c.cluster.DialServer(patchedContext, c.server, &schema.Endpoint_Port{Name: portName})
			if err != nil {
				fmt.Fprintf(console.Debug(patchedContext), "failed to dial orchestrator: %v\n", err)
				return nil, err
			}

			return conn, nil
		}),
	)
}

func getAwsConf(ctx context.Context, env cfg.Context) (*awsconf.Configuration, error) {
	sesh, err := awsprovider.ConfiguredSession(ctx, env.Configuration())
	if err != nil {
		return nil, err
	}
	if sesh == nil {
		return nil, nil
	}

	// Attach short term AWS credentials if configured for the current env.
	cfg := sesh.Config()
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, err
	}
	if creds.SessionToken != "" {
		if creds.Expired() {
			return nil, fmt.Errorf("aws credentials expired")
		}

		return &awsconf.Configuration{
			Region: cfg.Region,
			Static: &awsconf.Credentials{
				AccessKeyId:     creds.AccessKeyID,
				Expiration:      timestamppb.New(creds.Expires),
				SecretAccessKey: creds.SecretAccessKey,
				SessionToken:    creds.SessionToken,
			},
		}, nil
	}

	// TODO do we need to configure MFA here?
	result, err := sts.NewFromConfig(cfg).GetSessionToken(ctx, &sts.GetSessionTokenInput{})
	if err != nil {
		return nil, err
	}

	return &awsconf.Configuration{
		Region: cfg.Region,
		Static: &awsconf.Credentials{
			AccessKeyId:     aws.ToString(result.Credentials.AccessKeyId),
			Expiration:      timestamppb.New(aws.ToTime(result.Credentials.Expiration)),
			SecretAccessKey: aws.ToString(result.Credentials.SecretAccessKey),
			SessionToken:    aws.ToString(result.Credentials.SessionToken),
		},
	}, nil
}

func getUserAuth(ctx context.Context) (*auth.UserAuth, error) {
	x, err := auth.LoadUser()
	if err != nil {
		if errors.Is(err, auth.ErrRelogin) {
			// Don't require login yet. The orchestrator will fail with the appropriate error if required.
			return nil, nil
		}
		return nil, err
	}

	return x, nil
}

func CallDeploy(ctx context.Context, env cfg.Context, conn *grpc.ClientConn, plan *schema.DeployPlan) (string, error) {
	req := &proto.DeployRequest{
		Plan: plan,
	}

	if aws, err := getAwsConf(ctx, env); err != nil {
		return "", err
	} else if aws != nil {
		any := &anypb.Any{}
		if err := any.MarshalFrom(aws); err != nil {
			return "", err
		}
		req.Cfg = append(req.Cfg, any)
	}

	if cluster, ok := gke.ClusterConfigType.CheckGet(env.Configuration()); ok {
		any := &anypb.Any{}
		if err := any.MarshalFrom(cluster); err != nil {
			return "", err
		}
		req.Cfg = append(req.Cfg, any)
	}

	if project, ok := gcp.ProjectConfigType.CheckGet(env.Configuration()); ok {
		any := &anypb.Any{}
		if err := any.MarshalFrom(project); err != nil {
			return "", err
		}
		req.Cfg = append(req.Cfg, any)
	}

	auth, err := getUserAuth(ctx)
	if err != nil {
		return "", err
	}

	if auth != nil {
		authData, err := json.Marshal(auth)
		if err != nil {
			return "", fnerrors.InternalError("failed to marshal auth data: %w", err)
		}

		req.SerializedAuth = authData
		// Backwards compatible.
		req.Auth = &proto.InternalUserAuth{
			Username: auth.Username,
			Org:      auth.Org,
			Opaque:   auth.InternalOpaque,
		}
	}

	hostEnv, err := client.CheckGetHostEnv(env.Configuration())
	if err != nil {
		return "", err
	}
	req.HostEnv = hostEnv

	ctx, cancel := context.WithTimeout(ctx, connTimeout)
	defer cancel()

	resp, err := proto.NewOrchestrationServiceClient(conn).Deploy(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Id, nil
}

func WireDeploymentStatus(ctx context.Context, conn *grpc.ClientConn, id string, ch chan *orchestration.Event) error {
	if ch != nil {
		defer close(ch)
	}

	maxLogLevel := viper.GetInt32("console_log_level")
	req := &proto.DeploymentStatusRequest{
		Id:       id,
		LogLevel: maxLogLevel,
	}

	stream, err := proto.NewOrchestrationServiceClient(conn).DeploymentStatus(ctx, req)
	if err != nil {
		return err
	}

	for {
		in, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if ch != nil && in.Event != nil {
			ch <- in.Event
		}

		if in.Log != nil {
			forwardsLogs(ctx, maxLogLevel, in.Log)
		}
	}
}

func forwardsLogs(ctx context.Context, maxLogLevel int32, log *protolog.Log) {
	if l := log.Lines; l != nil {
		for _, line := range l.Lines {
			outputType := common.CatOutputType(l.Cat)
			if outputType == common.CatOutputDebug {
				// Call console.NamedDebug to respect DebugToConsole
				fmt.Fprintln(console.NamedDebug(ctx, l.Name), string(line))
			} else {
				fmt.Fprintln(console.TypedOutput(ctx, l.Name, outputType), string(line))
			}
		}
	}

	if log.Task != nil && log.LogLevel <= maxLogLevel {
		sink := tasks.SinkFrom(ctx)

		ra := tasks.ActionFromProto(ctx, "orchestrator", log.Task)

		switch log.Purpose {
		case protolog.Log_PURPOSE_WAITING:
			sink.Waiting(ra)
		case protolog.Log_PURPOSE_STARTED:
			sink.Started(ra)
		case protolog.Log_PURPOSE_DONE:
			sink.Done(ra)
		case protolog.Log_PURPOSE_INSTANT:
			sink.Instant(&ra.Data)
		default:
			fmt.Fprintf(console.Warnings(ctx), "unknown orchestrator log purpose %s", log.Purpose)
		}
	}
}
