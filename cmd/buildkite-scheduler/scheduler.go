package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
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

	m         sync.Mutex
	jobStates map[string]jobState
}

type jobState struct {
	processedAt time.Time
}

func newScheduler(agentToken, apiToken string) *scheduler {
	return &scheduler{
		agentToken: agentToken,
		apiToken:   apiToken,
		discovery:  make(chan *buildkite.Job),
		jobStates:  make(map[string]jobState),
	}
}

func (s *scheduler) runWorker(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case j := <-s.discovery:
			go s.processScheduledJob(ctx, j)
		}
	}
}

func (s *scheduler) processScheduledJob(ctx context.Context, j *buildkite.Job) {
	s.m.Lock()
	newJob := false
	state := s.jobStates[*j.ID]
	if state.processedAt.IsZero() {
		newJob = true
		state.processedAt = time.Now()
		s.jobStates[*j.ID] = state
	}
	s.m.Unlock()

	if !newJob || *j.State != "scheduled" {
		// already processed
		log.Printf("Skipping job %q", *j.ID)
		return
	}

	log.Printf("Starting job %q", *j.ID)
	if err := s.startJob(ctx, j); err != nil {
		log.Printf("Failed to start job %q: %v", *j.ID, err)
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
			if len(jobs) > 0 {
				log.Printf("Poll discovered %d jobs", len(jobs))
				for _, j := range jobs {
					s.discovery <- j
				}
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
	case *buildkite.JobFinishedEvent:
		return s.onJobFinished(ctx, event.(*buildkite.JobFinishedEvent))
	default:
		return false, nil
	}
}
func (s *scheduler) onJobScheduled(ctx context.Context, event *buildkite.JobScheduledEvent) (bool, error) {
	log.Printf("Webhook received job %q", *event.Job.ID)
	s.discovery <- event.Job
	return true, nil
}

func (s *scheduler) onJobFinished(ctx context.Context, event *buildkite.JobFinishedEvent) (bool, error) {
	s.m.Lock()
	state := s.jobStates[*event.Job.ID]
	s.m.Unlock()

	log.Printf("Finished job %q metrics:", *event.Job.ID)
	log.Printf("  scheduled: %v", event.Job.ScheduledAt.Format(time.StampMilli))
	log.Printf("  processed: %v (+%v)", state.processedAt.Format(time.StampMilli), state.processedAt.Sub(event.Job.ScheduledAt.Time))
	log.Printf("  vm ready: unknown")
	log.Printf("  started: %v (+%v)", event.Job.StartedAt.Format(time.StampMilli), event.Job.StartedAt.Sub(event.Job.ScheduledAt.Time))
	return false, nil
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
