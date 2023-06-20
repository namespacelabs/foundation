package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/shurcooL/graphql"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

const pollInterval = 2 * time.Second

type scheduler struct {
	agentToken string
	apiToken   string

	discovery chan *buildkite.Job
	jobState  map[string]bool
}

func newScheduler(agentToken, apiToken string) *scheduler {
	return &scheduler{
		agentToken: agentToken,
		apiToken:   apiToken,
		discovery:  make(chan *buildkite.Job),
		jobState:   make(map[string]bool),
	}
}

func (s *scheduler) runWorker(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case j := <-s.discovery:
			if s.jobState[*j.ID] || *j.State != "scheduled" {
				// already processed
				log.Printf("Skipping job %q", *j.ID)
				continue
			}
			log.Printf("Starting job %q", *j.ID)
			if err := s.startJob(ctx, j); err != nil {
				log.Printf("Failed to start job %q: %v", *j.ID, err)
			}
			s.jobState[*j.ID] = true
		}
	}
}

func (s *scheduler) runPoller(ctx context.Context) error {
	metrics, err := getMetrics(ctx, http.DefaultClient, s.agentToken)
	if err != nil {
		return fmt.Errorf("failed to get agent metrics: %w", err)
	}
	log.Printf("Polling jobs in org %q", metrics.Organization.Slug)

	clientConfig, err := buildkite.NewTokenConfig(s.apiToken, false)
	if err != nil {
		return fmt.Errorf("failed to prepare client: %w", err)
	}

	qlClient := graphql.NewClient("https://graphql.buildkite.com/v1", clientConfig.Client())
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
			jobs, err := queryJobs(ctx, qlClient, metrics.Organization.Slug)
			if err != nil {
				return fmt.Errorf("failed to query jobs: %w", err)
			}
			log.Printf("Poll discovered %d jobs", len(jobs))
			for _, j := range jobs {
				s.discovery <- j
			}
		}
	}
}

func (s *scheduler) startJob(ctx context.Context, j *buildkite.Job) error {
	container := &api.ContainerRequest{
		Name:  "buildkite-agent",
		Image: "buildkite/agent:3",
		Args: []string{
			"start",
			"--token", s.agentToken,
			"--acquire-job", *j.ID,
		},
		Env:            nil,
		Flag:           []string{"TERMINATE_ON_EXIT"},
		DockerSockPath: "/var/run/docker.sock",
	}
	req := api.CreateContainersRequest{
		MachineType: "2x8",
		Container:   []*api.ContainerRequest{container},
		Label: []*api.LabelEntry{
			{
				Name:  "buildkite-job",
				Value: *j.ID,
			},
		},
	}
	var response api.CreateContainersResponse
	if err := api.Endpoint.CreateContainers.Do(ctx, req, fnapi.DecodeJSONResponse(&response)); err != nil {
		return err
	}
	log.Printf("Created VM %q: %v", response.ClusterId, response.ClusterUrl)
	return nil
}

func (s *scheduler) onWebHook(ctx context.Context, event interface{}) (bool, error) {
	switch event.(type) {
	case *buildkite.PingEvent:
		return true, nil
	case *buildkite.JobScheduledEvent:
		return s.onJobScheduled(ctx, event.(*buildkite.JobScheduledEvent))
	default:
		return false, nil
	}
}
func (s *scheduler) onJobScheduled(ctx context.Context, event *buildkite.JobScheduledEvent) (bool, error) {
	log.Printf("Webhook received job %q", *event.Job.ID)
	s.discovery <- event.Job
	return true, nil
}

func webhookHandler(webhookSecret []byte, callback func(ctx context.Context, event interface{}) (bool, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("webhook: got connection: %v", r.URL)

		payload, err := buildkite.ValidatePayload(r, webhookSecret)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			log.Printf("webhook: forbidden: %v", err)
			return
		}

		typ := buildkite.WebHookType(r)
		event, err := buildkite.ParseWebHook(typ, payload)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Printf("webhook: bad payload for %s: %v", typ, err)
			return
		}
		log.Printf("webhook: got %s", typ)

		if ok, err := callback(r.Context(), event); err != nil {
			log.Printf("webhook: error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Internal error.\n")
			return
		} else if !ok {
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprintf(w, "We don't handle events of type %q\n", typ)
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "Thanks for the event.")
		}
	})
}

// func pollWithPing() {
// 	al := agentlogger.NewConsoleLogger(agentlogger.NewTextPrinter(os.Stderr), os.Exit)
// 	agentClient := agentapi.NewClient(al, agentapi.Config{})
// 	agentClient.Annotate()
// }
