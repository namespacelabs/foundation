// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/chisel/share/cnet"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func DialPort(ctx context.Context, cluster *KubernetesCluster, targetPort int) (net.Conn, error) {
	token, err := fnapi.FetchTenantToken(ctx)
	if err != nil {
		return nil, err
	}

	return DialPortWithToken(ctx, token, cluster, targetPort)
}

func DialPortWithToken(ctx context.Context, token *auth.Token, cluster *KubernetesCluster, targetPort int) (net.Conn, error) {
	d := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	hdrs := http.Header{}
	hdrs.Add("Authorization", token.BearerToken())

	wsConn, _, err := d.DialContext(ctx, fmt.Sprintf("wss://gate.%s/%s/%d", cluster.IngressDomain, cluster.ClusterId, targetPort), hdrs)
	if err != nil {
		return nil, err
	}

	return cnet.NewWebSocketConn(wsConn), nil
}
