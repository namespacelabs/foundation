// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package registry

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	dockertypes "github.com/docker/cli/cli/config/types"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type tokenProducerFunc func(context.Context) ([]types.AuthorizationData, error)
type makeServerAddressFunc func(context.Context) (string, error)

func refreshAuth(ctx context.Context, login tokenProducerFunc, makeServerAddress makeServerAddressFunc) (*dockertypes.AuthConfig, error) {
	authData, err := login(ctx)
	if err != nil {
		return nil, err
	}

	if len(authData) == 0 {
		return nil, fnerrors.RemoteError("aws/ecr: expected at least one authorization data back")
	}

	if authData[0].AuthorizationToken == nil {
		return nil, fnerrors.RemoteError("aws/ecr: expected the authorization tokens to be set")
	}

	decoded, err := base64.StdEncoding.DecodeString(*authData[0].AuthorizationToken)
	if err != nil {
		return nil, fnerrors.RemoteError("aws/ecr: failed to decode authorization token: %w", err)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) < 2 {
		return nil, fnerrors.RemoteError("aws/ecr: unexpected authorization token format")
	}

	serverAddr, err := makeServerAddress(ctx)
	if err != nil {
		return nil, err
	}

	return &dockertypes.AuthConfig{
		Username:      parts[0],
		Password:      parts[1],
		ServerAddress: serverAddr,
	}, nil
}