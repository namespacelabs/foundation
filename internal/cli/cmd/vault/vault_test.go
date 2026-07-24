// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestReadExportEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.env")
	if err := os.WriteFile(path, []byte("# Vault references\n\nKEY=sec_one\nFOOBAR = sec_two\n"), 0600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	got, err := readExportEntries(path)
	if err != nil {
		t.Fatalf("readExportEntries() error = %v", err)
	}
	want := []exportEntry{
		{name: "KEY", value: "sec_one"},
		{name: "FOOBAR", value: "sec_two"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readExportEntries() = %#v, want %#v", got, want)
	}
}

func TestReadExportEntriesRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		contents string
	}{
		{name: "empty", contents: "# Nothing here\n"},
		{name: "invalid name", contents: "1KEY=sec_one\n"},
		{name: "not a vault reference", contents: "KEY=value\n"},
		{name: "duplicate name", contents: "KEY=sec_one\nKEY=sec_two\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.env")
			if err := os.WriteFile(path, []byte(test.contents), 0600); err != nil {
				t.Fatalf("os.WriteFile() error = %v", err)
			}

			if _, err := readExportEntries(path); err == nil {
				t.Fatal("readExportEntries() error = nil, want error")
			}
		})
	}
}

func TestWriteExportFile(t *testing.T) {
	entries := []exportEntry{
		{name: "KEY", value: "sec_one"},
		{name: "FOOBAR", value: "sec_two"},
		{name: "SAME", value: "sec_one"},
	}
	calls := map[string]int{}
	resolve := func(_ context.Context, secretID string) (string, error) {
		calls[secretID]++
		return map[string]string{
			"sec_one": "resolved-one",
			"sec_two": "resolved-two",
		}[secretID], nil
	}

	path, err := writeExportFile(context.Background(), entries, resolve, 0)
	if err != nil {
		t.Fatalf("writeExportFile() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if got, want := string(contents), "KEY=resolved-one\nFOOBAR=resolved-two\nSAME=resolved-one\n"; got != want {
		t.Fatalf("export file contents = %q, want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0600); got != want {
		t.Fatalf("export file mode = %o, want %o", got, want)
	}
	if calls["sec_one"] != 1 {
		t.Fatalf("sec_one resolve calls = %d, want 1", calls["sec_one"])
	}
}

func TestResolveWithRetry(t *testing.T) {
	attempts := 0
	resolve := func(context.Context, string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("temporarily unavailable")
		}
		return "resolved", nil
	}

	got, err := resolveWithRetry(context.Background(), "sec_one", resolve, 0)
	if err != nil {
		t.Fatalf("resolveWithRetry() error = %v", err)
	}
	if got != "resolved" {
		t.Fatalf("resolveWithRetry() = %q, want resolved", got)
	}
	if attempts != 3 {
		t.Fatalf("resolve attempts = %d, want 3", attempts)
	}
}

func TestResolveWithRetryStopsOnPermissionDenied(t *testing.T) {
	attempts := 0
	resolve := func(context.Context, string) (string, error) {
		attempts++
		return "", status.Error(codes.PermissionDenied, "not authorized")
	}

	_, err := resolveWithRetry(context.Background(), "sec_one", resolve, time.Hour)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("resolveWithRetry() error = %v, want PermissionDenied", err)
	}
	if attempts != 1 {
		t.Fatalf("resolve attempts = %d, want 1", attempts)
	}
}

func TestQuoteShellPath(t *testing.T) {
	if got, want := quoteShellPath("/tmp/nsc-vault-export-123"), "/tmp/nsc-vault-export-123"; got != want {
		t.Fatalf("quoteShellPath() = %q, want %q", got, want)
	}
	if got, want := quoteShellPath("/tmp/it's here"), `'/tmp/it'"'"'s here'`; got != want {
		t.Fatalf("quoteShellPath() = %q, want %q", got, want)
	}
}
