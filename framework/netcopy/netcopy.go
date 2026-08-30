// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package netcopy

import (
	"io"
	"net"
	"sync"

	"inet.af/tcpproxy"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
)

type DebugLogFunc func(string, ...any)

func CopyConns(debuglog DebugLogFunc, dst, src io.ReadWriteCloser) error {
	var wg sync.WaitGroup
	errs := make([]error, 2)

	// We wait for both streams to handle half-close proxying.
	wg.Add(2)

	go func() {
		defer wg.Done()
		errs[0] = Copy(debuglog, src, dst)
	}()

	go func() {
		defer wg.Done()
		errs[1] = Copy(debuglog, dst, src)
	}()

	wg.Wait()

	if debuglog != nil {
		debuglog("copyconns done")
	}

	return multierr.New(errs...)
}

func Copy(debuglog DebugLogFunc, dst, src io.ReadWriteCloser) error {
	// Before we unwrap src and/or dst, copy any buffered data.
	if wc, ok := src.(*tcpproxy.Conn); ok && len(wc.Peeked) > 0 {
		if _, err := dst.Write(wc.Peeked); err != nil {
			return err
		}

		wc.Peeked = nil
	}

	// Unwrap the src and dst from *Conn to *net.TCPConn so Go
	// 1.11's splice optimization kicks in.
	src = underlyingConn(src)
	dst = underlyingConn(dst)

	if debuglog != nil {
		debuglog("proxy copy: %s --> %s", addr(src), addr(dst))
	}

	_, err := io.Copy(dst, src)
	if debuglog != nil {
		debuglog("proxy copy: %s --> %s: got %v", addr(src), addr(dst), err)
	}

	closeErr := closeWrite(dst)
	if debuglog != nil {
		debuglog("%s: close dst write %v", addr(dst), closeErr)
	}

	return multierr.New(err, closeErr)
}

func addr(src io.ReadWriteCloser) string {
	if c, ok := src.(interface {
		RemoteAddrDebug() string
	}); ok {
		return c.RemoteAddrDebug()
	}

	if c, ok := src.(interface {
		RemoteAddr() net.Addr
	}); ok {
		return c.RemoteAddr().String()
	}

	return "?"
}

func underlyingConn(c io.ReadWriteCloser) io.ReadWriteCloser {
	if wrap, ok := c.(*tcpproxy.Conn); ok {
		return wrap.Conn
	}
	return c
}

func closeWrite(conn io.ReadWriteCloser) error {
	if c, ok := conn.(*net.TCPConn); ok {
		return c.CloseWrite()
	}

	if c, ok := conn.(*net.UnixConn); ok {
		return c.CloseWrite()
	}

	return conn.Close()
}
