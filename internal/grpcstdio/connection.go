// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpcstdio

import (
	"io"
	"net"
	"time"
)

func NewConnection(in io.Writer, out io.Reader) net.Conn {
	return stdioConn{in, out}
}

type stdioConn struct {
	in  io.Writer
	out io.Reader
}

func (s stdioConn) Read(b []byte) (int, error) {
	return s.out.Read(b)
}

func (s stdioConn) Write(b []byte) (int, error) {
	return s.in.Write(b)
}

func (s stdioConn) Close() error {
	// close context.
	return nil
}

func (s stdioConn) LocalAddr() net.Addr {
	return stdioAddr{network: "stdio", str: "local"}
}

func (s stdioConn) RemoteAddr() net.Addr {
	return stdioAddr{network: "stdio", str: "remote"}
}

func (s stdioConn) SetDeadline(t time.Time) error {
	// Not supported.
	return nil
}
func (s stdioConn) SetReadDeadline(t time.Time) error {
	// Not supported.
	return nil
}
func (s stdioConn) SetWriteDeadline(t time.Time) error {
	// Not supported.
	return nil
}

type stdioAddr struct {
	network string
	str     string
}

func (d stdioAddr) Network() string {
	return d.network
}

func (d stdioAddr) String() string {
	return d.str
}
