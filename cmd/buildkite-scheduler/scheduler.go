package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/buildkite/go-buildkite/v3/buildkite"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

type scheduler struct {
	agentToken string
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
	container := &api.ContainerRequest{
		Name:  "buildkite-agent",
		Image: "buildkite/agent:3",
		Args: []string{
			"start",
			"--token", s.agentToken,
			"--acquire-job", *event.Job.ID,
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
				Value: *event.Job.ID,
			},
		},
	}
	var response api.CreateContainersResponse
	if err := api.Endpoint.CreateContainers.Do(ctx, req, fnapi.DecodeJSONResponse(&response)); err != nil {
		return false, err
	}
	log.Printf("Created VM %q: %v", response.ClusterId, response.ClusterUrl)
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
		log.Printf("webhook: got %s %s", typ, string(payload))

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
