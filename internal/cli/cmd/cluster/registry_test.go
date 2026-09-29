// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	registryv1beta "namespacelabs.dev/integrations/proto/namespace/cloud/registry/v1beta"
	"namespacelabs.dev/integrations/proto/namespace/stdlib"
)

type registryListClient struct {
	registryv1beta.ContainerRegistryServiceClient
	images       func(*registryv1beta.ListImagesRequest) (*registryv1beta.ListImagesResponse, error)
	repositories func(*registryv1beta.ListRepositoriesRequest) (*registryv1beta.ListRepositoriesResponse, error)
}

func (c registryListClient) ListImages(_ context.Context, req *registryv1beta.ListImagesRequest, _ ...grpc.CallOption) (*registryv1beta.ListImagesResponse, error) {
	return c.images(req)
}

func (c registryListClient) ListRepositories(_ context.Context, req *registryv1beta.ListRepositoriesRequest, _ ...grpc.CallOption) (*registryv1beta.ListRepositoriesResponse, error) {
	return c.repositories(req)
}

func TestListRegistryRepositories(t *testing.T) {
	wantErr := errors.New("page failed")
	for _, tt := range []struct {
		name   string
		limit  int
		max    []int64
		want   []string
		failAt int
	}{
		{name: "all pages including empty", max: []int64{1000, 1000, 1000}, want: []string{"first", "last"}},
		{name: "limit above page size", limit: 2000, max: []int64{1000, 1000, 1000}, want: []string{"first", "last"}},
		{name: "limit at page size", limit: 1000, max: []int64{1000, 999, 999}, want: []string{"first", "last"}},
		{name: "limit on first page", limit: 1, max: []int64{1}, want: []string{"first"}},
		{name: "limit across pages", limit: 2, max: []int64{2, 1, 1}, want: []string{"first", "last"}},
		{name: "fewer than limit", limit: 5, max: []int64{5, 4, 4}, want: []string{"first", "last"}},
		{name: "later page error", max: []int64{1000, 1000}, failAt: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pages := []*registryv1beta.ListRepositoriesResponse{
				{Repositories: []*registryv1beta.Repository{{Name: "first"}}, PaginationCursor: []byte("second")},
				{PaginationCursor: []byte("third")},
				{Repositories: []*registryv1beta.Repository{{Name: "last"}}},
			}
			cursors := []string{"", "second", "third"}
			calls := 0
			client := registryListClient{repositories: func(req *registryv1beta.ListRepositoriesRequest) (*registryv1beta.ListRepositoriesResponse, error) {
				i := calls
				calls++
				if i >= len(tt.max) {
					t.Fatal("unexpected extra request")
				}
				if string(req.PaginationCursor) != cursors[i] || req.MaxEntries != tt.max[i] {
					t.Fatalf("request %d: got %v, want cursor %q, max %d", i, req, cursors[i], tt.max[i])
				}
				if calls == tt.failAt {
					return nil, wantErr
				}
				return pages[i], nil
			}}
			got, err := listRegistryRepositories(context.Background(), client, tt.limit)
			if tt.failAt > 0 {
				if !errors.Is(err, wantErr) || got != nil {
					t.Fatalf("got %v, %v; want no results and page error", got, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, repo := range got {
				names = append(names, repo.Name)
			}
			if !reflect.DeepEqual(names, tt.want) || calls != len(tt.max) {
				t.Fatalf("got %v in %d calls; want %v in %d calls", names, calls, tt.want, len(tt.max))
			}
		})
	}
}

func TestListRegistryImages(t *testing.T) {
	wantErr := errors.New("page failed")
	for _, tt := range []struct {
		name           string
		limit          int
		includeDeleted bool
		repository     string
		max            []int64
		want           []string
		failAt         int
	}{
		{name: "all pages", max: []int64{10000, 10000, 10000, 10000}, want: []string{"live1", "live2", "live3"}},
		{name: "limit above page size", limit: 20000, max: []int64{10000, 10000, 10000, 10000}, want: []string{"live1", "live2", "live3"}},
		{name: "limit at page size", limit: 10000, max: []int64{10000, 10000, 10000, 10000}, want: []string{"live1", "live2", "live3"}},
		{name: "filtered pages and repository", repository: "app", limit: 2, max: []int64{2, 2, 2}, want: []string{"live1", "live2"}},
		{name: "include deleted", includeDeleted: true, limit: 3, max: []int64{3, 2}, want: []string{"deleted1", "live1", "deleted2"}},
		{name: "fewer than limit", limit: 9, max: []int64{9, 9, 9, 9}, want: []string{"live1", "live2", "live3"}},
		{name: "later page error", max: []int64{10000, 10000, 10000}, failAt: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pages := []*registryv1beta.ListImagesResponse{
				{Images: []*registryv1beta.Image{{Digest: "deleted1", DeletedAt: timestamppb.Now()}}, PaginationCursor: []byte("second")},
				{Images: []*registryv1beta.Image{{Digest: "live1"}, {Digest: "deleted2", DeletedAt: timestamppb.Now()}}, PaginationCursor: []byte("third")},
				{Images: []*registryv1beta.Image{{Digest: "live2"}}, PaginationCursor: []byte("fourth")},
				{Images: []*registryv1beta.Image{{Digest: "live3"}}},
			}
			cursors := []string{"", "second", "third", "fourth"}
			calls := 0
			client := registryListClient{images: func(req *registryv1beta.ListImagesRequest) (*registryv1beta.ListImagesResponse, error) {
				i := calls
				calls++
				if i >= len(tt.max) {
					t.Fatal("unexpected extra request")
				}
				if string(req.PaginationCursor) != cursors[i] || req.MaxEntries != tt.max[i] {
					t.Fatalf("request %d: got %v, want cursor %q, max %d", i, req, cursors[i], tt.max[i])
				}
				if tt.repository == "" {
					if req.MatchRepository != nil {
						t.Fatalf("unexpected repository filter: %v", req.MatchRepository)
					}
				} else if req.MatchRepository.GetOp() != stdlib.StringMatcher_IS_ANY_OF || !reflect.DeepEqual(req.MatchRepository.GetValues(), []string{tt.repository}) {
					t.Fatalf("repository filter not preserved: %v", req.MatchRepository)
				}
				if calls == tt.failAt {
					return nil, wantErr
				}
				if int64(len(pages[i].Images)) > req.MaxEntries {
					t.Fatal("fixture exceeds requested page size")
				}
				return pages[i], nil
			}}
			req := &registryv1beta.ListImagesRequest{}
			if tt.repository != "" {
				req.MatchRepository = &stdlib.StringMatcher{Values: []string{tt.repository}, Op: stdlib.StringMatcher_IS_ANY_OF}
			}
			got, err := listRegistryImages(context.Background(), client, req, tt.includeDeleted, tt.limit)
			if tt.failAt > 0 {
				if !errors.Is(err, wantErr) || got != nil {
					t.Fatalf("got %v, %v; want no results and page error", got, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			var digests []string
			for _, img := range got {
				digests = append(digests, img.Digest)
			}
			if !reflect.DeepEqual(digests, tt.want) || calls != len(tt.max) {
				t.Fatalf("got %v in %d calls; want %v in %d calls", digests, calls, tt.want, len(tt.max))
			}
		})
	}
}

func TestRegistryFullPageBoundaries(t *testing.T) {
	for _, kind := range []struct {
		name string
		cap  int
	}{
		{name: "images", cap: 10000},
		{name: "repositories", cap: 1000},
	} {
		for _, tt := range []struct {
			name  string
			limit int
			want  int
			max   []int64
		}{
			{name: "unlimited", want: kind.cap + 11, max: []int64{int64(kind.cap), int64(kind.cap)}},
			{name: "at boundary", limit: kind.cap, want: kind.cap, max: []int64{int64(kind.cap)}},
			{name: "across boundary", limit: kind.cap + 7, want: kind.cap + 7, max: []int64{int64(kind.cap), 7}},
		} {
			t.Run(kind.name+"/"+tt.name, func(t *testing.T) {
				calls := 0
				nextPage := func(cursor []byte, max int64) (int, int, []byte) {
					t.Helper()
					i := calls
					calls++
					if i >= len(tt.max) || max != tt.max[i] {
						t.Fatalf("request %d: unexpected max %d", i, max)
					}
					if i == 0 {
						if len(cursor) != 0 {
							t.Fatalf("initial cursor = %q", cursor)
						}
						return 0, kind.cap, []byte("opaque-next-page")
					}
					if string(cursor) != "opaque-next-page" {
						t.Fatalf("continuation cursor = %q", cursor)
					}
					if tt.limit > 0 {
						return kind.cap, kind.cap + 7, []byte("more-results")
					}
					return kind.cap, kind.cap + 11, nil
				}
				client := registryListClient{
					images: func(req *registryv1beta.ListImagesRequest) (*registryv1beta.ListImagesResponse, error) {
						if !reflect.DeepEqual(req.MatchRepository.GetValues(), []string{"app"}) || req.MatchRepository.GetOp() != stdlib.StringMatcher_IS_ANY_OF {
							t.Fatalf("repository filter not preserved: %v", req.MatchRepository)
						}
						start, end, cursor := nextPage(req.PaginationCursor, req.MaxEntries)
						resp := &registryv1beta.ListImagesResponse{PaginationCursor: cursor}
						for i := start; i < end; i++ {
							resp.Images = append(resp.Images, &registryv1beta.Image{Repository: "app", Digest: fmt.Sprint(i)})
						}
						return resp, nil
					},
					repositories: func(req *registryv1beta.ListRepositoriesRequest) (*registryv1beta.ListRepositoriesResponse, error) {
						start, end, cursor := nextPage(req.PaginationCursor, req.MaxEntries)
						resp := &registryv1beta.ListRepositoriesResponse{PaginationCursor: cursor}
						for i := start; i < end; i++ {
							resp.Repositories = append(resp.Repositories, &registryv1beta.Repository{Name: fmt.Sprint(i)})
						}
						return resp, nil
					},
				}
				var got []string
				if kind.name == "images" {
					req := &registryv1beta.ListImagesRequest{MatchRepository: &stdlib.StringMatcher{Values: []string{"app"}, Op: stdlib.StringMatcher_IS_ANY_OF}}
					images, err := listRegistryImages(context.Background(), client, req, true, tt.limit)
					if err != nil {
						t.Fatal(err)
					}
					for _, img := range images {
						got = append(got, img.Digest)
					}
				} else {
					repos, err := listRegistryRepositories(context.Background(), client, tt.limit)
					if err != nil {
						t.Fatal(err)
					}
					for _, repo := range repos {
						got = append(got, repo.Name)
					}
				}
				if len(got) != tt.want || calls != len(tt.max) {
					t.Fatalf("got %d entries in %d calls, want %d in %d", len(got), calls, tt.want, len(tt.max))
				}
				for i, value := range got {
					if value != fmt.Sprint(i) {
						t.Fatalf("entry %d = %q", i, value)
					}
				}
			})
		}
	}
}

func TestListRegistryImagesDeletedPages(t *testing.T) {
	calls := 0
	client := registryListClient{images: func(req *registryv1beta.ListImagesRequest) (*registryv1beta.ListImagesResponse, error) {
		calls++
		if calls > 2 || req.MaxEntries != 5000 {
			t.Fatalf("request %d: max = %d, want full 5000-entry pages", calls, req.MaxEntries)
		}
		resp := &registryv1beta.ListImagesResponse{PaginationCursor: []byte("more")}
		for i := 0; i < 5000; i++ {
			img := &registryv1beta.Image{Digest: fmt.Sprint((calls-1)*5000 + i)}
			if (calls == 1 && i == 4999) || (calls == 2 && i < 4998) {
				img.DeletedAt = timestamppb.Now()
			}
			resp.Images = append(resp.Images, img)
		}
		return resp, nil
	}}
	images, err := listRegistryImages(context.Background(), client, &registryv1beta.ListImagesRequest{}, false, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(images) != 5000 {
		t.Fatalf("got %d images in %d calls, want 5000 in 2", len(images), calls)
	}
	for i, img := range images {
		want := fmt.Sprint(i)
		if i == 4999 {
			want = "9998"
		}
		if img.DeletedAt != nil || img.Digest != want {
			t.Fatalf("image %d = %v, want live image %s", i, img, want)
		}
	}
}

func TestRegistryListTimeFlagValidation(t *testing.T) {
	for _, flag := range []string{"created_after", "created_before", "expires_after", "expires_before"} {
		for _, tt := range []struct {
			args []string
			want string
		}{
			{args: []string{"--" + flag, "invalid"}, want: "invalid " + strings.SplitN(flag, "_", 2)[1] + " timestamp"},
			{args: []string{"--repositories", "--" + flag, "2026-09-20T00:00:00Z"}, want: "none of the others can be"},
		} {
			cmd := newRegistryListCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)
			if err := cmd.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("args %v: got %v, want %q", tt.args, err, tt.want)
			}
		}
	}
}

func TestListRegistryImagesTimeFilters(t *testing.T) {
	created := &stdlib.TimestampRange{After: timestamppb.New(time.Unix(100, 123)), Before: timestamppb.New(time.Unix(200, 456))}
	expires := &stdlib.TimestampRange{After: timestamppb.New(time.Unix(300, 789)), Before: timestamppb.New(time.Unix(500, 987))}
	req := &registryv1beta.ListImagesRequest{
		CreatedAt: proto.Clone(created).(*stdlib.TimestampRange),
		ExpiresAt: proto.Clone(expires).(*stdlib.TimestampRange),
	}
	calls := 0
	client := registryListClient{images: func(req *registryv1beta.ListImagesRequest) (*registryv1beta.ListImagesResponse, error) {
		calls++
		if calls > 3 {
			t.Fatal("unexpected extra request")
		}
		// Check the actual protobuf wire representation, including the new field.
		wire, err := proto.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		decoded := &registryv1beta.ListImagesRequest{}
		if err := proto.Unmarshal(wire, decoded); err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(decoded.CreatedAt, created) || !proto.Equal(decoded.ExpiresAt, expires) {
			t.Fatalf("request %d changed timestamp filters: %v", calls, decoded)
		}
		if calls < 3 {
			return &registryv1beta.ListImagesResponse{PaginationCursor: []byte(fmt.Sprint(calls))}, nil
		}
		return &registryv1beta.ListImagesResponse{Images: []*registryv1beta.Image{{Digest: "match"}}}, nil
	}}
	images, err := listRegistryImages(context.Background(), client, req, false, 1)
	if err != nil || calls != 3 || len(images) != 1 || images[0].Digest != "match" {
		t.Fatalf("got %v, %v in %d calls", images, err, calls)
	}
}
