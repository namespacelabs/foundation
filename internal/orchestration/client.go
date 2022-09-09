// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/orchestration/proto"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const (
	serverPkg   = "namespacelabs.dev/foundation/internal/orchestration/server"
	connTimeout = time.Minute // TODO reduce - we've seen slow connections in CI
)

var (
	UseOrchestrator              = true
	RenderOrchestratorDeployment = false
)

type RemoteOrchestrator struct {
	cluster  runtime.Cluster
	server   *schema.Server
	endpoint *schema.Endpoint
}

type clientInstance struct {
	ctx     planning.Context
	cluster runtime.Cluster

	compute.DoScoped[*RemoteOrchestrator] // Only connect once per configuration.
}

func ensureOrchestrator(env planning.Context, cluster runtime.Cluster) compute.Computable[*RemoteOrchestrator] {
	return &clientInstance{ctx: env, cluster: cluster}
}

func (c *clientInstance) Action() *tasks.ActionEvent {
	return tasks.Action("orchestrator.ensure").Arg("env", c.ctx.Environment().Name)
}

func (c *clientInstance) Inputs() *compute.In {
	return compute.Inputs().Str("env", c.ctx.Environment().Name)
}

func (c *clientInstance) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (c *clientInstance) Compute(ctx context.Context, _ compute.Resolved) (*RemoteOrchestrator, error) {
	env := makeOrchEnv(c.ctx)

	cluster := c.cluster.Rebind(env)

	focus, err := provision.RequireServer(ctx, env, schema.PackageName(serverPkg))
	if err != nil {
		return nil, err
	}

	plan, err := deploy.PrepareDeployServers(ctx, env, cluster, []provision.Server{focus}, nil)
	if err != nil {
		return nil, err
	}

	computed, err := compute.GetValue(ctx, plan)
	if err != nil {
		return nil, err
	}

	waiters, err := ops.Execute(ctx, runtime.TaskServerDeploy, env, computed.Deployer)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, connTimeout)
	defer cancel()

	if RenderOrchestratorDeployment {
		if err := deploy.Wait(ctx, env, cluster, waiters); err != nil {
			return nil, err
		}
	} else {
		if err := ops.WaitMultiple(ctx, waiters, nil); err != nil {
			return nil, err
		}
	}

	var endpoint *schema.Endpoint
	for _, e := range computed.ComputedStack.Endpoints {
		if e.ServerOwner != serverPkg {
			continue
		}

		for _, m := range e.ServiceMetadata {
			if m.Kind == proto.OrchestrationService_ServiceDesc.ServiceName {
				endpoint = e
			}
		}
	}

	if endpoint == nil {
		return nil, fnerrors.InternalError("orchestration service not found: %+v", computed.ComputedStack.Endpoints)
	}

	return &RemoteOrchestrator{cluster: cluster, server: focus.Proto(), endpoint: endpoint}, nil
}

func (c *RemoteOrchestrator) Connect(ctx context.Context) (*grpc.ClientConn, error) {
	// Make sure we dial with the parent context (as opposed to a grpc managed context).
	conn, err := c.cluster.DialServer(ctx, c.server, c.endpoint.Port.ContainerPort)
	if err != nil {
		return nil, err
	}

	return grpc.DialContext(ctx, "orchestrator",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return conn, nil
		}),
	)
}

func getAwsConf(ctx context.Context, env planning.Context) (*awsprovider.Conf, error) {
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
	if creds.CanExpire {
		if creds.Expired() {
			return nil, fmt.Errorf("aws credentials expired")
		}
		return &awsprovider.Conf{
			Region: cfg.Region,
			Static: &awsprovider.Credentials{
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

	return &awsprovider.Conf{
		Region: cfg.Region,
		Static: &awsprovider.Credentials{
			AccessKeyId:     aws.ToString(result.Credentials.AccessKeyId),
			Expiration:      timestamppb.New(aws.ToTime(result.Credentials.Expiration)),
			SecretAccessKey: aws.ToString(result.Credentials.SecretAccessKey),
			SessionToken:    aws.ToString(result.Credentials.SessionToken),
		},
	}, nil
}

func getUserAuth(ctx context.Context) (*fnapi.UserAuth, error) {
	auth, err := fnapi.LoadUser()
	if err != nil {
		if errors.Is(err, fnapi.ErrRelogin) {
			// Don't require login yet. The orchestrator will fail with the appropriate error if required.
			return nil, nil
		}
		return nil, err
	}

	res, err := fnapi.GetSessionToken(ctx, string(auth.Opaque), time.Hour)
	if err != nil {
		return nil, err
	}

	auth.Opaque = []byte(res.Token)

	return auth, nil
}

func CallDeploy(ctx context.Context, env planning.Context, conn *grpc.ClientConn, plan *schema.DeployPlan) (string, error) {
	req := &proto.DeployRequest{
		Plan: plan,
	}

	var err error
	if req.Aws, err = getAwsConf(ctx, env); err != nil {
		return "", err
	}

	if req.Auth, err = getUserAuth(ctx); err != nil {
		return "", err
	}

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

	req := &proto.DeploymentStatusRequest{
		Id: id,
	}

	stream, err := proto.NewOrchestrationServiceClient(conn).DeploymentStatus(ctx, req)
	if err != nil {
		return err
	}

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if ch != nil && in.Event != nil {
			ch <- in.Event
		}
	}
}
