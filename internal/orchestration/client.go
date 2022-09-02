// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const (
	serverPkg  = "namespacelabs.dev/foundation/internal/orchestration/server"
	servicePkg = "namespacelabs.dev/foundation/internal/orchestration/service"
)

var (
	UseOrchestrator              = false
	RenderOrchestratorDeployment = false
)

type clientInstance struct {
	env provision.Env

	compute.DoScoped[proto.OrchestrationServiceClient] // Only connect once per configuration.
}

func ConnectToClient(env provision.Env) compute.Computable[proto.OrchestrationServiceClient] {
	return &clientInstance{env: env}
}

func (c *clientInstance) Action() *tasks.ActionEvent {
	return tasks.Action("orchestrator.connect").Arg("env", c.env.Name())
}

func (c *clientInstance) Inputs() *compute.In {
	return compute.Inputs().Str("env", c.env.Name())
}

func (c *clientInstance) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (c *clientInstance) Compute(ctx context.Context, _ compute.Resolved) (proto.OrchestrationServiceClient, error) {
	focus, err := c.env.RequireServer(ctx, schema.PackageName(serverPkg))
	if err != nil {
		return nil, err
	}

	plan, err := deploy.PrepareDeployServers(ctx, c.env, []provision.Server{focus}, nil)
	if err != nil {
		return nil, err
	}

	computed, err := compute.GetValue(ctx, plan)
	if err != nil {
		return nil, err
	}

	waiters, err := computed.Deployer.Execute(ctx, runtime.TaskServerDeploy, c.env)
	if err != nil {
		return nil, err
	}

	if RenderOrchestratorDeployment {
		if err := deploy.Wait(ctx, c.env, waiters); err != nil {
			return nil, err
		}
	} else {
		if err := ops.WaitMultiple(ctx, waiters, nil); err != nil {
			return nil, err
		}
	}

	endpoint := &schema.Endpoint{}
	for _, e := range computed.ComputedStack.Endpoints {
		if e.EndpointOwner != servicePkg {
			continue
		}
		for _, meta := range e.ServiceMetadata {
			if meta.Kind == proto.OrchestrationService_ServiceDesc.ServiceName {
				endpoint = e
			}
		}
	}

	rt := runtime.For(ctx, c.env)

	portch := make(chan runtime.ForwardedPort)

	defer close(portch)
	if _, err := rt.ForwardPort(ctx, focus.Proto(), endpoint.Port.ContainerPort, []string{"127.0.0.1"}, func(fp runtime.ForwardedPort) {
		portch <- fp
	}); err != nil {
		return nil, err
	}

	port, ok := <-portch
	if !ok {
		return nil, fnerrors.InternalError("didn't receive forwarded port from orchestration server")
	}

	conn, err := grpc.DialContext(ctx, fmt.Sprintf("127.0.0.1:%d", port.LocalPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fnerrors.InternalError("unable to connect to orchestration server: %w", err)
	}

	cli := proto.NewOrchestrationServiceClient(conn)

	return cli, nil
}

func getAwsConf(ctx context.Context, env provision.Env) (*awsprovider.Conf, error) {
	sesh, err := awsprovider.ConfiguredSession(ctx, env.DevHost(), devhost.ByEnvironment(env.Proto()))
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

func Deploy(ctx context.Context, env provision.Env, plan *schema.DeployPlan) (string, error) {
	cli, err := compute.GetValue(ctx, ConnectToClient(env))
	if err != nil {
		return "", err
	}

	req := &proto.DeployRequest{
		Plan: plan,
	}

	if req.Aws, err = getAwsConf(ctx, env); err != nil {
		return "", err
	}

	if req.Auth, err = getUserAuth(ctx); err != nil {
		return "", err
	}

	resp, err := cli.Deploy(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Id, nil
}

func WireDeploymentStatus(ctx context.Context, env provision.Env, id string, ch chan *orchestration.Event) error {
	if ch != nil {
		defer close(ch)
	}

	req := &proto.DeploymentStatusRequest{
		Id: id,
	}

	cli, err := compute.GetValue(ctx, ConnectToClient(env))
	if err != nil {
		return err
	}

	stream, err := cli.DeploymentStatus(ctx, req)
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
