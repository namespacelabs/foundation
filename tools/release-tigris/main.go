// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const defaultEndpoint = "https://t3.storage.dev"

var tarballRE = regexp.MustCompile(`^(ns|nsc)_([^_]+)_(darwin|linux)_(amd64|arm64)\.tar\.gz$`)

type manifest struct {
	Tool        string             `json:"tool"`
	Version     string             `json:"version"`
	PublishedAt time.Time          `json:"published_at"`
	Artifacts   []manifestArtifact `json:"artifacts"`
}

type manifestArtifact struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	SHA256   string `json:"sha256"`
}

func main() {
	ctx := context.Background()

	if len(os.Args) < 2 {
		fatalf("usage: %s <release|installers> [flags]", os.Args[0])
	}

	switch os.Args[1] {
	case "release":
		if err := runRelease(ctx, os.Args[2:]); err != nil {
			fatalf("release upload failed: %v", err)
		}
	case "installers":
		if err := runInstallers(ctx, os.Args[2:]); err != nil {
			fatalf("installer upload failed: %v", err)
		}
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func runRelease(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	bucket := fs.String("bucket", "", "Tigris bucket name")
	endpoint := fs.String("endpoint", defaultEndpoint, "Tigris S3 endpoint")
	distDir := fs.String("dist-dir", "dist", "GoReleaser dist directory")
	tag := fs.String("tag", "", "Release tag, for example v0.0.123")
	fs.Parse(args)

	if *bucket == "" {
		return errors.New("bucket is required")
	}
	if *tag == "" {
		return errors.New("tag is required")
	}

	version := strings.TrimPrefix(*tag, "v")
	checksums, err := readChecksums(filepath.Join(*distDir, "checksums.txt"))
	if err != nil {
		return err
	}

	manifests, tarballs, err := buildManifests(*distDir, version, *tag, time.Now().UTC(), checksums)
	if err != nil {
		return err
	}

	client, err := newClient(ctx, *endpoint)
	if err != nil {
		return err
	}

	for _, file := range tarballs {
		if err := putFile(ctx, client, *bucket, path.Join("foundation", "releases", *tag, filepath.Base(file)), file, "application/gzip"); err != nil {
			return err
		}
	}

	checksumsPath := filepath.Join(*distDir, "checksums.txt")
	if err := putFile(ctx, client, *bucket, path.Join("foundation", "releases", *tag, "checksums.txt"), checksumsPath, "text/plain; charset=utf-8"); err != nil {
		return err
	}

	for _, tool := range []string{"ns", "nsc"} {
		payload, err := json.MarshalIndent(manifests[tool], "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s manifest: %w", tool, err)
		}

		versionKey := path.Join("foundation", "releases", *tag, tool+".json")
		latestKey := path.Join("foundation", "releases", "latest", tool+".json")
		if err := putBytes(ctx, client, *bucket, versionKey, payload, "application/json"); err != nil {
			return err
		}
		if err := putBytes(ctx, client, *bucket, latestKey, payload, "application/json"); err != nil {
			return err
		}
	}

	return nil
}

func runInstallers(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("installers", flag.ExitOnError)
	bucket := fs.String("bucket", "", "Tigris bucket name")
	endpoint := fs.String("endpoint", defaultEndpoint, "Tigris S3 endpoint")
	fs.Parse(args)

	if *bucket == "" {
		return errors.New("bucket is required")
	}

	client, err := newClient(ctx, *endpoint)
	if err != nil {
		return err
	}

	for _, installer := range []string{"install.sh", "install_nsc.sh"} {
		localPath := filepath.Join("install", installer)
		remotePath := path.Join("foundation", "install", installer)
		if err := putFile(ctx, client, *bucket, remotePath, localPath, "text/plain; charset=utf-8"); err != nil {
			return err
		}
	}

	return nil
}

func buildManifests(distDir, version, tag string, publishedAt time.Time, checksums map[string]string) (map[string]manifest, []string, error) {
	entries, err := os.ReadDir(distDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read dist dir: %w", err)
	}

	manifests := map[string]manifest{
		"ns":  {Tool: "ns", Version: tag, PublishedAt: publishedAt},
		"nsc": {Tool: "nsc", Version: tag, PublishedAt: publishedAt},
	}

	var tarballs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		match := tarballRE.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}

		tool, artifactVersion, osName, arch := match[1], match[2], strings.ToUpper(match[3]), strings.ToUpper(match[4])
		if artifactVersion != version {
			return nil, nil, fmt.Errorf("unexpected version in %s: got %s want %s", entry.Name(), artifactVersion, version)
		}

		sha256, ok := checksums[entry.Name()]
		if !ok {
			return nil, nil, fmt.Errorf("missing checksum for %s", entry.Name())
		}

		artifact := manifestArtifact{
			Filename: entry.Name(),
			OS:       osName,
			Arch:     arch,
			SHA256:   sha256,
		}

		manifest := manifests[tool]
		manifest.Artifacts = append(manifest.Artifacts, artifact)
		manifests[tool] = manifest
		tarballs = append(tarballs, filepath.Join(distDir, entry.Name()))
	}

	for _, tool := range []string{"ns", "nsc"} {
		manifest := manifests[tool]
		if len(manifest.Artifacts) == 0 {
			return nil, nil, fmt.Errorf("no artifacts found for %s", tool)
		}

		sort.Slice(manifest.Artifacts, func(i, j int) bool {
			return manifest.Artifacts[i].Filename < manifest.Artifacts[j].Filename
		})
		manifests[tool] = manifest
	}

	sort.Strings(tarballs)
	return manifests, tarballs, nil
}

func readChecksums(checksumsPath string) (map[string]string, error) {
	file, err := os.Open(checksumsPath)
	if err != nil {
		return nil, fmt.Errorf("open checksums.txt: %w", err)
	}
	defer file.Close()

	checksums := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected checksum line %q", line)
		}

		checksums[parts[1]] = parts[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums.txt: %w", err)
	}

	return checksums, nil
}

func newClient(ctx context.Context, endpoint string) (*s3.Client, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               endpoint,
				SigningRegion:     "auto",
				HostnameImmutable: true,
			}, nil
		}

		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = true
	}), nil
}

func putFile(ctx context.Context, client *s3.Client, bucket, key, filename, contentType string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open %s: %w", filename, err)
	}
	defer file.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload %s to s3://%s/%s: %w", filename, bucket, key, err)
	}

	return nil
}

func putBytes(ctx context.Context, client *s3.Client, bucket, key string, body []byte, contentType string) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(body),
		ContentLength: aws.Int64(int64(len(body))),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload s3://%s/%s: %w", bucket, key, err)
	}

	return nil
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
