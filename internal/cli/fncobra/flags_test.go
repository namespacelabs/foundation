// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/integrations/proto/namespace/stdlib"
)

func TestDurationSet(t *testing.T) {
	var value time.Duration
	d := duration{&value}

	if err := d.Set("2w"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if value != 14*24*time.Hour {
		t.Errorf("got %v, want %v", value, 14*24*time.Hour)
	}
}

func TestDurationString(t *testing.T) {
	val := 24 * time.Hour
	d := duration{&val}

	if got := d.String(); got != "24h0m0s" {
		t.Errorf("got %q, want %q", got, "24h0m0s")
	}
}

func TestDurationType(t *testing.T) {
	var d duration
	if got := d.Type(); got != "duration" {
		t.Errorf("got %q, want %q", got, "duration")
	}
}

func TestDurationVar(t *testing.T) {
	var (
		d     time.Duration
		flags pflag.FlagSet
	)
	DurationVar(&flags, &d, "test", 24*time.Hour, "test")

	if d != 24*time.Hour {
		t.Errorf("got %v, want %v", d, 24*time.Hour)
	}

	flags.Set("test", "2w")

	if d != 14*24*time.Hour {
		t.Errorf("got %v, want %v", d, 14*24*time.Hour)
	}
}

func TestDuration(t *testing.T) {
	var flags pflag.FlagSet

	d := Duration(&flags, "test", 24*time.Hour, "test")

	if *d != 24*time.Hour {
		t.Errorf("got %v, want %v", *d, 24*time.Hour)
	}

	flags.Set("test", "2w")

	if *d != 14*24*time.Hour {
		t.Errorf("got %v, want %v", *d, 14*24*time.Hour)
	}
}

func TestParseTimestampRange(t *testing.T) {
	after := timestamppb.New(time.Date(2026, 9, 20, 8, 15, 0, 123456789, time.UTC))
	before := timestamppb.New(time.Date(2026, 9, 22, 11, 30, 0, 0, time.UTC))
	for _, tt := range []struct {
		name          string
		after, before *string
		want          *stdlib.TimestampRange
	}{
		{name: "nil bounds"},
		{name: "empty bounds", after: proto.String(""), before: proto.String("")},
		{name: "nil and empty", before: proto.String("")},
		{name: "after with offset and nanos", after: proto.String("2026-09-20T10:15:00.123456789+02:00"), want: &stdlib.TimestampRange{After: after}},
		{name: "before only", before: proto.String("2026-09-22T11:30:00Z"), want: &stdlib.TimestampRange{Before: before}},
		{name: "both", after: proto.String("2026-09-20T08:15:00.123456789Z"), before: proto.String("2026-09-22T11:30:00Z"), want: &stdlib.TimestampRange{After: after, Before: before}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestampRange(tt.before, tt.after)
			if err != nil || !proto.Equal(got, tt.want) {
				t.Fatalf("got %v, %v; want %v", got, err, tt.want)
			}
		})
	}
	for _, tt := range []struct{ after, before, want string }{
		{after: "yesterday", want: "invalid after timestamp"},
		{before: "2026-09-20", want: "invalid before timestamp"},
		{after: "0000-01-01T00:00:00Z", want: "invalid after timestamp"},
		{after: "2026-09-22T00:00:00Z", before: "2026-09-20T00:00:00Z", want: "after must be earlier than before"},
		{after: "2026-09-20T02:00:00+02:00", before: "2026-09-20T00:00:00Z", want: "after must be earlier than before"},
	} {
		_, err := ParseTimestampRange(&tt.before, &tt.after)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("range %q, %q: got %v, want %q", tt.after, tt.before, err, tt.want)
		}
	}
}
