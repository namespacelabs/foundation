package main

import (
	"context"
	"io"
	"net"

	"namespacelabs.dev/breakpoint/pkg/quicproxyclient"
)

type ProxyListener struct {
	ch     chan net.Conn
	cancel func()
}

func StartProxyListener(baseCtx context.Context, rendezvouz string, onAllocation func(endpoint string)) net.Listener {
	ctx, cancel := context.WithCancel(baseCtx)
	l := &ProxyListener{make(chan net.Conn), cancel}

	go func() {
		quicproxyclient.Serve(ctx, *rendezvouzEndpoint, nil, quicproxyclient.Handlers{
			OnAllocation: onAllocation,
			Proxy: func(conn net.Conn) error {
				l.ch <- conn
				return nil
			},
		})
		close(l.ch)
	}()
	return l
}

// Accept waits for and returns the next connection to the listener.
func (l *ProxyListener) Accept() (net.Conn, error) {
	conn, ok := <-l.ch
	if !ok {
		return nil, io.ErrClosedPipe
	}
	return conn, nil
}

func (l *ProxyListener) Close() error {
	l.cancel()
	return nil
}

func (l *ProxyListener) Addr() net.Addr {
	return noneAddr{}
}

type noneAddr struct{}

func (noneAddr) Network() string { return "none" }
func (noneAddr) String() string  { return "none" }
