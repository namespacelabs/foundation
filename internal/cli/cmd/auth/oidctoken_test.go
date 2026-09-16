// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"testing"
	"time"
)

func TestExchangeOIDCTokenDurationFlag(t *testing.T) {
	cmd := NewExchangeOIDCTokenCmd()
	if err := cmd.Flags().Set("duration", "2h"); err != nil {
		t.Fatal(err)
	}

	flag := cmd.Flags().Lookup("duration")
	if flag == nil {
		t.Fatal("missing --duration")
	}
	if got := flag.Value.String(); got != (2 * time.Hour).String() {
		t.Fatalf("duration = %q, want %q", got, 2*time.Hour)
	}
}
