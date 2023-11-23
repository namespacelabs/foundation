// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/chisel/share/cnet"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

func DialPort(ctx context.Context, cluster *KubernetesCluster, targetPort int) (net.Conn, error) {
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	return DialPortWithToken(ctx, token, cluster, targetPort)
}

func DialEndpoint(ctx context.Context, endpoint string, opts ...Option) (net.Conn, error) {
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	return DialEndpointWithToken(ctx, token, endpoint, opts...)
}

func DialPortWithToken(ctx context.Context, token fnapi.Token, cluster *KubernetesCluster, targetPort int) (net.Conn, error) {
	return DialEndpointWithToken(ctx, token, fmt.Sprintf("wss://gate.%s/%s/%d", cluster.IngressDomain, cluster.ClusterId, targetPort))
}

func DialHostedServiceWithToken(ctx context.Context, token fnapi.Token, cluster *KubernetesCluster, serviceName string, vars url.Values) (net.Conn, error) {
	u := url.URL{
		Scheme:   "wss",
		Host:     fmt.Sprintf("gate.%s", cluster.IngressDomain),
		Path:     fmt.Sprintf("/%s/hsvc.%s", cluster.ClusterId, serviceName),
		RawQuery: vars.Encode(),
	}

	return DialEndpointWithToken(ctx, token, u.String())
}

type Option func(*dialOptions)

type dialOptions struct {
	refreshClusterID string
}

func WithRefresh(clusterID string) Option {
	return func(do *dialOptions) {
		do.refreshClusterID = clusterID
	}
}

func DialEndpointWithToken(ctx context.Context, token fnapi.Token, endpoint string, opts ...Option) (net.Conn, error) {
	tid := ids.NewRandomBase32ID(4)
	fmt.Fprintf(console.Debug(ctx), "[%s] Gateway: dialing %v...\n", tid, endpoint)

	d := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	bt, err := fnapi.BearerToken(ctx, token)
	if err != nil {
		return nil, err
	}

	hdrs := http.Header{}
	hdrs.Add("Authorization", "Bearer "+bt)

	var o dialOptions
	for _, opt := range opts {
		opt(&o)
	}

	t := time.Now()
	wsConn, _, err := d.DialContext(ctx, endpoint, hdrs)
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "[%s] Gateway: %v: failed: %v\n", tid, endpoint, err)
		return nil, err
	}

	fmt.Fprintf(console.Debug(ctx), "[%s] Gateway: dialing %v... took %v\n", tid, endpoint, time.Since(t))

	conn := cnet.NewWebSocketConn(wsConn)

	if o.refreshClusterID != "" {
		fmt.Fprintf(console.Debug(ctx), "[%s] starting background refresh: %s\n", tid, o.refreshClusterID)
		cancel := StartBackgroundRefreshing(ctx, o.refreshClusterID)

		return forwardClose{conn, cancel}, nil
	}

	return conn, nil
}

func StartBackgroundRefreshing(ctx context.Context, clusterID string) func() {
	sink := tasks.SinkFrom(ctx)
	backgroundCtx := tasks.WithSink(context.Background(), sink)
	ctx, done := context.WithCancel(backgroundCtx)

	tasks.Action("endpoint.cluster.refresh").Arg("cluster_id", clusterID).LogLevel(1).LogToSink(sink)

	go func() {
		_ = StartRefreshing(ctx, Methods, clusterID, func(err error) error {
			if !errors.Is(err, context.Canceled) {
				fmt.Fprintf(console.Warnings(ctx), "Failed to refresh cluster: %v\n", err)
			}
			return nil
		})

		tasks.Action("endpoint.cluster.refresh.done").Arg("cluster_id", clusterID).LogLevel(1).LogToSink(sink)
	}()

	return done
}

type forwardClose struct {
	net.Conn
	close func()
}

func (an forwardClose) Close() error {
	an.close()
	return an.Conn.Close()
}
