// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package naming

import (
	"fmt"
	"time"

	"github.com/miekg/dns"
)

// Inspired by go-acme/dns01, MIT licensed.

type Resolver struct {
	Timeout     time.Duration
	Nameservers []string
}

type ResolveResult struct {
	a, aaaa, cname string
}

func (r *ResolveResult) A() string {
	if r == nil {
		return ""
	}
	return r.a
}

func (r *ResolveResult) CNAME() string {
	if r == nil {
		return ""
	}
	return r.cname
}

func (r *ResolveResult) String() string {
	if r.a != "" {
		return "A:" + r.a
	}
	if r.aaaa != "" {
		return "AAAA:" + r.a
	}
	if r.cname != "" {
		return "CNAME:" + r.a
	}
	return ""
}

func (d Resolver) Lookup(fqdn string) (*ResolveResult, error) {
	// XXX we should query for TypeAAAA as well.
	in, err := d.dnsQuery(fqdn, dns.TypeA, d.Nameservers, true)
	if err != nil {
		return nil, err
	}

	if in == nil {
		return nil, nil
	}

	switch in.Rcode {
	case dns.RcodeSuccess:
		for _, ans := range in.Answer {
			if cname, ok := ans.(*dns.CNAME); ok {
				return &ResolveResult{cname: cname.Target}, nil
			}
		}

		for _, ans := range in.Answer {
			if r, ok := ans.(*dns.A); ok {
				return &ResolveResult{a: r.A.String()}, nil
			}
		}

		for _, ans := range in.Answer {
			if r, ok := ans.(*dns.AAAA); ok {
				return &ResolveResult{a: r.AAAA.String()}, nil
			}
		}

	case dns.RcodeNameError:
		// NXDOMAIN
	default:
		// Any response code other than NOERROR and NXDOMAIN is treated as error
		return nil, fmt.Errorf("unexpected response code '%s' for %s", dns.RcodeToString[in.Rcode], fqdn)
	}

	return nil, nil
}

func (d Resolver) sendDNSQuery(m *dns.Msg, ns string) (*dns.Msg, error) {
	udp := &dns.Client{Net: "udp", Timeout: d.Timeout}
	in, _, err := udp.Exchange(m, ns)

	if in != nil && in.Truncated {
		tcp := &dns.Client{Net: "tcp", Timeout: d.Timeout}
		// If the TCP request succeeds, the err will reset to nil
		in, _, err = tcp.Exchange(m, ns)
	}

	return in, err
}

func createDNSMsg(fqdn string, rtype uint16, recursive bool) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(fqdn), rtype)
	m.SetEdns0(4096, false)

	if !recursive {
		m.RecursionDesired = false
	}

	return m
}

func (d Resolver) dnsQuery(fqdn string, rtype uint16, nameservers []string, recursive bool) (*dns.Msg, error) {
	m := createDNSMsg(fqdn, rtype, recursive)

	var in *dns.Msg
	var err error

	for _, ns := range nameservers {
		in, err = d.sendDNSQuery(m, ns)
		if err == nil && len(in.Answer) > 0 {
			break
		}
	}
	return in, err
}
