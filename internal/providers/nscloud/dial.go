// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nscloud

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/chisel/share/cnet"
)

func DialPort(ctx context.Context, cluster *KubernetesCluster, targetPort int) (net.Conn, error) {
	d := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	wsConn, _, err := d.DialContext(ctx, fmt.Sprintf("wss://proxy-%s.int-%s/nodeport/%d", cluster.ClusterId, cluster.IngressDomain, targetPort), nil)
	if err != nil {
		return nil, err
	}

	return cnet.NewWebSocketConn(wsConn), nil
}
