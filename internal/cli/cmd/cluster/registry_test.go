// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
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

func collectRegistryResults[T any](entries iter.Seq2[T, error]) ([]T, error) {
	var results []T
	for entry, err := range entries {
		if err != nil {
			return results, err
		}
		results = append(results, entry)
	}
	return results, nil
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
		{name: "later page error", max: []int64{1000, 1000}, failAt: 2, want: []string{"first"}},
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
			got, err := collectRegistryResults(listRegistryRepositories(context.Background(), client, tt.limit))
			if tt.failAt > 0 {
				if !errors.Is(err, wantErr) {
					t.Fatalf("got %v; want page error", err)
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
		{name: "limit at page size", limit: 10000, max: []int64{10000, 10000, 9999, 9998}, want: []string{"live1", "live2", "live3"}},
		{name: "filtered pages and repository", repository: "app", limit: 2, max: []int64{2, 2, 1}, want: []string{"live1", "live2"}},
		{name: "include deleted", includeDeleted: true, limit: 3, max: []int64{3, 2}, want: []string{"deleted1", "live1", "deleted2"}},
		{name: "include deleted unlimited", includeDeleted: true, max: []int64{10000, 10000, 10000, 10000}, want: []string{"deleted1", "live1", "deleted2", "live2", "live3"}},
		{name: "fewer than limit", limit: 9, max: []int64{9, 9, 8, 7}, want: []string{"live1", "live2", "live3"}},
		{name: "later page error", max: []int64{10000, 10000, 10000}, failAt: 3, want: []string{"live1"}},
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
				wantMode := registryv1beta.ListImagesRequest_EXCLUDE_DELETED
				if tt.includeDeleted {
					wantMode = registryv1beta.ListImagesRequest_INCLUDE_DELETED
				}
				if req.DeletionFilter != wantMode {
					t.Fatalf("request %d: deletion mode = %v, want %v", i, req.DeletionFilter, wantMode)
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
				if !tt.includeDeleted {
					var live []*registryv1beta.Image
					for _, img := range pages[i].Images {
						if img.DeletedAt == nil {
							live = append(live, img)
						}
					}
					pages[i].Images = live
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
			got, err := collectRegistryResults(listRegistryImages(context.Background(), client, req, tt.includeDeleted, tt.limit))
			if tt.failAt > 0 {
				if !errors.Is(err, wantErr) {
					t.Fatalf("got %v; want page error", err)
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
		name           string
		cap            int
		includeDeleted bool
	}{
		{name: "images", cap: 10000},
		{name: "images including deleted", cap: 10000, includeDeleted: true},
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
							img := &registryv1beta.Image{Repository: "app", Digest: fmt.Sprint(i)}
							if kind.includeDeleted && i%2 == 0 {
								img.DeletedAt = timestamppb.New(time.Unix(1, 0))
							}
							resp.Images = append(resp.Images, img)
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
				if kind.name != "repositories" {
					req := &registryv1beta.ListImagesRequest{MatchRepository: &stdlib.StringMatcher{Values: []string{"app"}, Op: stdlib.StringMatcher_IS_ANY_OF}}
					images, err := collectRegistryResults(listRegistryImages(context.Background(), client, req, kind.includeDeleted, tt.limit))
					if err != nil {
						t.Fatal(err)
					}
					for _, img := range images {
						got = append(got, img.Digest)
					}
				} else {
					repos, err := collectRegistryResults(listRegistryRepositories(context.Background(), client, tt.limit))
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

func TestListRegistryImagesRemainingLimit(t *testing.T) {
	calls := 0
	client := registryListClient{images: func(req *registryv1beta.ListImagesRequest) (*registryv1beta.ListImagesResponse, error) {
		calls++
		wantMax := int64(5000)
		if calls == 2 {
			wantMax = 1
		}
		if calls > 2 || req.MaxEntries != wantMax || req.DeletionFilter != registryv1beta.ListImagesRequest_EXCLUDE_DELETED {
			t.Fatalf("request %d: got %v, want max %d and EXCLUDE_DELETED", calls, req, wantMax)
		}
		resp := &registryv1beta.ListImagesResponse{PaginationCursor: []byte("more")}
		if calls == 1 {
			for i := 0; i < 4999; i++ {
				resp.Images = append(resp.Images, &registryv1beta.Image{Digest: fmt.Sprint(i)})
			}
		} else {
			resp.Images = []*registryv1beta.Image{{Digest: "4999"}}
		}
		return resp, nil
	}}
	images, err := collectRegistryResults(listRegistryImages(context.Background(), client, &registryv1beta.ListImagesRequest{}, false, 5000))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(images) != 5000 {
		t.Fatalf("got %d images in %d calls, want 5000 in 2", len(images), calls)
	}
	for i, img := range images {
		want := fmt.Sprint(i)
		if img.DeletedAt != nil || img.Digest != want {
			t.Fatalf("image %d = %v, want live image %s", i, img, want)
		}
	}
}

func TestRegistryListLimitFlags(t *testing.T) {
	cmd := newRegistryListCmd()
	if limit, err := cmd.Flags().GetInt("limit"); err != nil || limit != 100 {
		t.Fatalf("default limit = %d, %v; want 100", limit, err)
	}
	if noLimit, err := cmd.Flags().GetBool("no_limit"); err != nil || noLimit {
		t.Fatalf("default no_limit = %v, %v; want false", noLimit, err)
	}
	for _, tt := range []struct {
		args []string
		want string
	}{
		{args: []string{"--limit", "0"}, want: "--limit must be positive; use --no_limit"},
		{args: []string{"--limit", "-1"}, want: "--limit must be positive; use --no_limit"},
		{args: []string{"--limit", "100", "--no_limit"}, want: "none of the others can be"},
		// Invalid timestamps stop accepted limit options before authentication.
		{args: []string{"--no_limit", "--created_after", "invalid"}, want: "invalid after timestamp"},
		{args: []string{"--limit", "1", "--created_after", "invalid"}, want: "invalid after timestamp"},
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
	for _, mode := range []registryv1beta.ListImagesRequest_DeletionFilter{registryv1beta.ListImagesRequest_EXCLUDE_DELETED, registryv1beta.ListImagesRequest_INCLUDE_DELETED} {
		t.Run(mode.String(), func(t *testing.T) {
			created := &stdlib.TimestampRange{After: timestamppb.New(time.Unix(100, 123)), Before: timestamppb.New(time.Unix(200, 456))}
			expires := &stdlib.TimestampRange{After: timestamppb.New(time.Unix(300, 789)), Before: timestamppb.New(time.Unix(500, 987))}
			repository := &stdlib.StringMatcher{Op: stdlib.StringMatcher_IS_ANY_OF, Values: []string{"app"}}
			req := &registryv1beta.ListImagesRequest{
				CreatedAt:       proto.Clone(created).(*stdlib.TimestampRange),
				ExpiresAt:       proto.Clone(expires).(*stdlib.TimestampRange),
				MatchRepository: proto.Clone(repository).(*stdlib.StringMatcher),
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
				if !proto.Equal(decoded.CreatedAt, created) || !proto.Equal(decoded.ExpiresAt, expires) || !proto.Equal(decoded.MatchRepository, repository) || decoded.DeletionFilter != mode {
					t.Fatalf("request %d changed filters: %v", calls, decoded)
				}
				if calls < 3 {
					return &registryv1beta.ListImagesResponse{PaginationCursor: []byte(fmt.Sprint(calls))}, nil
				}
				return &registryv1beta.ListImagesResponse{Images: []*registryv1beta.Image{{Digest: "match"}}}, nil
			}}
			images, err := collectRegistryResults(listRegistryImages(context.Background(), client, req, mode == registryv1beta.ListImagesRequest_INCLUDE_DELETED, 1))
			if err != nil || calls != 3 || len(images) != 1 || images[0].Digest != "match" {
				t.Fatalf("got %v, %v in %d calls", images, err, calls)
			}
		})
	}
}

type registryErrorWriter struct{ err error }

func (w registryErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRegistryStreamingOutput(t *testing.T) {
	wantErr := errors.New("stream failed")
	for _, kind := range []string{"images", "repositories"} {
		for _, output := range []string{"json", "table"} {
			for _, scenario := range []struct {
				name      string
				limit     int
				wantCalls int
				wantNames []string
			}{
				{name: "all pages", wantCalls: 3, wantNames: []string{"first", "longer-last"}},
				{name: "limit", limit: 1, wantCalls: 1, wantNames: []string{"first"}},
				{name: "empty", wantCalls: 1},
				{name: "page error", wantCalls: 2},
				{name: "write error", wantCalls: 1},
			} {
				t.Run(kind+"/"+output+"/"+scenario.name, func(t *testing.T) {
					var buf bytes.Buffer
					calls := 0
					nextPage := func() (string, []byte, error) {
						t.Helper()
						calls++
						if calls > scenario.wantCalls {
							t.Fatal("unexpected extra request")
						}
						if calls > 1 && !strings.Contains(buf.String(), "first") {
							t.Fatal("first page was not written before the next request")
						}
						if calls > 1 && !strings.HasSuffix(buf.String(), "\n") {
							t.Fatal("page output must be flushed through line-buffered consoles")
						}
						if scenario.name == "empty" {
							return "", nil, nil
						}
						if calls == 1 {
							return "first", []byte("second"), nil
						}
						if scenario.name == "page error" {
							return "", nil, wantErr
						}
						if calls == 2 {
							return "", []byte("third"), nil
						}
						return "longer-last", nil, nil
					}
					stamp := timestamppb.New(time.Date(2026, 9, 20, 8, 15, 0, 0, time.UTC))
					client := registryListClient{
						images: func(*registryv1beta.ListImagesRequest) (*registryv1beta.ListImagesResponse, error) {
							name, cursor, err := nextPage()
							resp := &registryv1beta.ListImagesResponse{PaginationCursor: cursor}
							if name != "" {
								resp.Images = []*registryv1beta.Image{{Repository: name, Digest: "sha256:abc", CreatedAt: stamp}}
							}
							return resp, err
						},
						repositories: func(*registryv1beta.ListRepositoriesRequest) (*registryv1beta.ListRepositoriesResponse, error) {
							name, cursor, err := nextPage()
							resp := &registryv1beta.ListRepositoriesResponse{PaginationCursor: cursor}
							if name != "" {
								resp.Repositories = []*registryv1beta.Repository{{Name: name, LastPush: stamp}}
							}
							return resp, err
						},
					}
					var w io.Writer = &buf
					if scenario.name == "write error" {
						w = registryErrorWriter{wantErr}
					}
					var err error
					if kind == "images" {
						err = printRegistryImages(w, "registry.example", listRegistryImages(context.Background(), client, &registryv1beta.ListImagesRequest{}, false, scenario.limit), output)
					} else {
						err = printRegistryRepositories(w, listRegistryRepositories(context.Background(), client, scenario.limit), output)
					}
					if calls != scenario.wantCalls {
						t.Fatalf("got %d requests, want %d", calls, scenario.wantCalls)
					}
					if strings.HasSuffix(scenario.name, "error") {
						if !errors.Is(err, wantErr) {
							t.Fatalf("got %v, want stream error", err)
						}
						if output == "json" && json.Valid(buf.Bytes()) {
							t.Fatal("failed listing must not look like a complete JSON result")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if output == "json" {
						var rows []map[string]any
						if err := json.Unmarshal(buf.Bytes(), &rows); err != nil || len(rows) != len(scenario.wantNames) {
							t.Fatalf("invalid JSON results: %s, %v", buf.String(), err)
						}
						for i, name := range scenario.wantNames {
							want := map[string]any{"name": name, "last_push": "2026-09-20T08:15:00Z"}
							if kind == "images" {
								want = map[string]any{"repository": name, "digest": "sha256:abc", "createdAt": "2026-09-20T08:15:00Z", "image_ref": "registry.example/" + name + "@sha256:abc"}
							}
							if !reflect.DeepEqual(rows[i], want) {
								t.Fatalf("row %d = %v, want %v", i, rows[i], want)
							}
						}
						if len(rows) == 0 && strings.TrimSpace(buf.String()) != "[]" {
							t.Fatalf("empty output = %q, want []", buf.String())
						}
					} else if len(scenario.wantNames) == 0 {
						if buf.String() != "No "+kind+" found.\n" {
							t.Fatalf("unexpected empty output: %q", buf.String())
						}
					} else {
						lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
						if len(lines) != len(scenario.wantNames)+1 {
							t.Fatalf("expected one header and %d rows: %q", len(scenario.wantNames), buf.String())
						}
						for i, name := range scenario.wantNames {
							if !strings.Contains(lines[i+1], name) || !strings.Contains(lines[i+1], "2026-09-20T08:15:00Z") {
								t.Fatalf("unexpected row: %q", lines[i+1])
							}
						}
					}
				})
			}
		}
	}
}
