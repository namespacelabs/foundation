package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/shurcooL/graphql"
)

type AgentMetrics struct {
	// XXX: There are more fields
	Organization struct {
		Slug string `json:"slug,omitempty"`
	} `json:"organization"`
}

// buildkite API docs: https://buildkite.com/docs/apis/agent-api/metrics
func getMetrics(ctx context.Context, client *http.Client, agentToken string) (*AgentMetrics, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://agent.buildkite.com/v3/metrics", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Token "+agentToken)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("metrics query returned status %d", resp.StatusCode)
	}
	var m AgentMetrics
	err = json.NewDecoder(resp.Body).Decode(&m)
	return &m, err
}

func queryJobs(ctx context.Context, qlClient *graphql.Client, orgSlug string) ([]*buildkite.Job, error) {
	var q struct {
		Organization struct {
			Pipelines struct {
				Edges []struct {
					Node struct {
						Slug   string
						Builds struct {
							Edges []struct {
								Node struct {
									Jobs struct {
										Edges []struct {
											Node struct {
												JobTypeCommand struct {
													UUID  string
													State string
												} `graphql:"... on JobTypeCommand"`
											}
										}
									} `graphql:"jobs(state: SCHEDULED, first: 500)"`
								}
							}
						} `graphql:"builds(first: 500)"`
					}
				}
			} `graphql:"pipelines(first: 500)"`
		} `graphql:"organization(slug: $orgSlug)"`
	}
	vars := map[string]interface{}{
		"orgSlug": orgSlug,
	}
	if err := qlClient.Query(ctx, &q, vars); err != nil {
		return nil, err
	}
	jobs := []*buildkite.Job{}
	for _, p := range q.Organization.Pipelines.Edges {
		for _, b := range p.Node.Builds.Edges {
			for _, j := range b.Node.Jobs.Edges {
				state := strings.ToLower(j.Node.JobTypeCommand.State)
				jobs = append(jobs, &buildkite.Job{
					// XXX: load more fields
					ID:    &j.Node.JobTypeCommand.UUID,
					State: &state,
				})
			}
		}
	}
	return jobs, nil
}
