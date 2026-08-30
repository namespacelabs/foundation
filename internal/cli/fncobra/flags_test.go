// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
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
