// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package downloader

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownloadMultiPart(t *testing.T) {
	const fileSize = 1024 * 1024 * 1024 // 1GB

	data := make([]byte, fileSize)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("failed to generate test data: %v", err)
	}

	hash := sha256.Sum256(data)
	expectedDigest := hex.EncodeToString(hash[:])

	var requestCount atomic.Int32
	var failureCount atomic.Int32
	failureCount.Store(2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodHead && failureCount.Load() > 0 {
			failureCount.Add(-1)
			http.Error(w, "simulated failure", http.StatusInternalServerError)
			return
		}

		rs := io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data)))
		http.ServeContent(w, r, "testfile", time.Now(), rs)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "download.bin")

	ctx := context.Background()
	opts := Options{
		ChunkSize:  10 * 1024 * 1024, // 10MB chunks
		Concurrent: 4,
		ResolveURL: func(ctx context.Context) (string, error) {
			return server.URL, nil
		},
	}

	if err := Download(ctx, destPath, opts); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	downloadedData, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if len(downloadedData) != fileSize {
		t.Errorf("downloaded file size mismatch: got %d, want %d", len(downloadedData), fileSize)
	}

	downloadedHash := sha256.Sum256(downloadedData)
	downloadedDigest := hex.EncodeToString(downloadedHash[:])

	if downloadedDigest != expectedDigest {
		t.Errorf("digest mismatch: got %s, want %s", downloadedDigest, expectedDigest)
	}

	if requestCount.Load() <= 3 {
		t.Errorf("expected multiple requests with retries, got %d", requestCount.Load())
	}

	t.Logf("Successfully downloaded 1GB file in %d requests (with retries)", requestCount.Load())
}

func TestDownloadWithResume(t *testing.T) {
	const fileSize = 50 * 1024 * 1024 // 50MB

	data := make([]byte, fileSize)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("failed to generate test data: %v", err)
	}

	hash := sha256.Sum256(data)
	expectedDigest := hex.EncodeToString(hash[:])

	var requestCount atomic.Int32
	var failFirstNRequests atomic.Int32
	var sleepCalls atomic.Int32
	failFirstNRequests.Store(100)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodHead && failFirstNRequests.Load() > 0 {
			failFirstNRequests.Add(-1)
			http.Error(w, "simulated failure", http.StatusServiceUnavailable)
			return
		}

		rs := io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data)))
		http.ServeContent(w, r, "testfile", time.Now(), rs)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "download.bin")
	stateFile := destPath + ".state"
	downloadFile := destPath + ".download"

	ctx := context.Background()
	opts := Options{
		ChunkSize:  5 * 1024 * 1024, // 5MB chunks
		Concurrent: 3,
		Resume:     true,
		ResolveURL: func(ctx context.Context) (string, error) {
			return server.URL, nil
		},
		sleepFunc: func(ctx context.Context, d time.Duration) error {
			sleepCalls.Add(1)
			return nil
		},
	}

	err := Download(ctx, destPath, opts)
	if err == nil {
		t.Fatalf("First download attempt should have failed")
	}
	t.Logf("First download failed as expected: %v", err)

	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("state file should exist after failed download: %v", err)
	}

	if _, err := os.Stat(downloadFile); err != nil {
		t.Fatalf("download file should exist after failed download: %v", err)
	}

	failFirstNRequests.Store(0)

	if err := Download(ctx, destPath, opts); err != nil {
		t.Fatalf("Resumed download failed: %v", err)
	}

	downloadedData, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if len(downloadedData) != fileSize {
		t.Errorf("downloaded file size mismatch: got %d, want %d", len(downloadedData), fileSize)
	}

	downloadedHash := sha256.Sum256(downloadedData)
	downloadedDigest := hex.EncodeToString(downloadedHash[:])

	if downloadedDigest != expectedDigest {
		t.Errorf("digest mismatch: got %s, want %s", downloadedDigest, expectedDigest)
	}

	if _, err := os.Stat(stateFile); err == nil {
		t.Errorf("state file should be deleted after successful download")
	}

	if _, err := os.Stat(downloadFile); err == nil {
		t.Errorf("download file should be deleted after successful download (renamed to dest)")
	}

	if sleepCalls.Load() == 0 {
		t.Errorf("expected retry backoff sleeps to occur during failures")
	}

	t.Logf("Successfully resumed and completed download in %d total requests with %d retry backoffs", requestCount.Load(), sleepCalls.Load())
}

func TestDownloadSingleStream(t *testing.T) {
	const fileSize = 10 * 1024 * 1024 // 10MB

	data := make([]byte, fileSize)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("failed to generate test data: %v", err)
	}

	hash := sha256.Sum256(data)
	expectedDigest := hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "download.bin")

	ctx := context.Background()
	opts := Options{
		ResolveURL: func(ctx context.Context) (string, error) {
			return server.URL, nil
		},
	}

	if err := Download(ctx, destPath, opts); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	downloadedData, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	downloadedHash := sha256.Sum256(downloadedData)
	downloadedDigest := hex.EncodeToString(downloadedHash[:])

	if downloadedDigest != expectedDigest {
		t.Errorf("digest mismatch: got %s, want %s", downloadedDigest, expectedDigest)
	}
}
