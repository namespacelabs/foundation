// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"net"
	"net/netip"
)

// Cloudflare's published edge IP ranges. Used to gate trust of the
// CF-Connecting-IP header so it is only honoured for requests that actually
// arrived from a CF edge.
//
// Source: https://www.cloudflare.com/ips/
var cloudflareCIDRs = []string{
	// IPv4
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
	// IPv6
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

var cloudflarePrefixes = func() []netip.Prefix {
	out := make([]netip.Prefix, 0, len(cloudflareCIDRs))
	for _, c := range cloudflareCIDRs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			panic("servercore: invalid Cloudflare CIDR " + c + ": " + err.Error())
		}
		out = append(out, p.Masked())
	}
	return out
}()

// isCloudflarePeer reports whether remoteAddr (typically r.RemoteAddr, in the
// "host:port" form) is within Cloudflare's published edge IP ranges.
func isCloudflarePeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}

	addr = addr.Unmap()
	for _, p := range cloudflarePrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
