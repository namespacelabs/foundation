// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/testdata/service/proto"
	"namespacelabs.dev/foundation/testing"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/std/testdata/service/post", "post")

		var metrics *schema.HttpExportedService
		for _, ie := range t.Request.InternalEndpoint {
			if ie.ServerOwner == "namespacelabs.dev/foundation/std/testdata/server/gogrpc" {
				for _, md := range ie.ServiceMetadata {
					if md.Kind == "prometheus.io/metrics" {
						metrics = &schema.HttpExportedService{}
						if err := md.Details.UnmarshalTo(metrics); err != nil {
							return err
						}
						break
					}
				}
			}
		}

		if metrics == nil {
			return errors.New("prometheus metrics endpoint missing")
		}

		conn, err := t.Connect(ctx, endpoint)
		if err != nil {
			return err
		}

		response, err := proto.NewPostServiceClient(conn).Post(ctx, &proto.PostRequest{Input: "Hello from the test"})
		if err != nil {
			return err
		}

		log.Println(response)

		scrapeUrl := testing.MakeHttpUrl(endpoint, metrics.Path)
		log.Printf("Scraping responses at: %s", scrapeUrl)

		resp, err := http.Get(scrapeUrl)
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		dec := expfmt.NewDecoder(resp.Body, expfmt.FmtText)
		mf := &dto.MetricFamily{}

		var m *dto.Metric
		for {
			if err := dec.Decode(mf); err == io.EOF {
				break
			} else if err != nil {
				return err
			}

			if mf.GetName() == "grpc_server_msg_received_total" {
				for _, metric := range mf.Metric {
					if hasLabels(metric.Label, map[string]string{
						"grpc_service": "std.testdata.service.proto.PostService",
						"grpc_method":  "Post",
					}) {
						m = metric
					} else {
						var labels []string
						for _, label := range metric.GetLabel() {
							labels = append(labels, fmt.Sprintf("%s:%s", label.GetName(), label.GetValue()))
						}
						log.Printf("Found other GRPC metric with labels: %s", strings.Join(labels, ","))
					}
				}
			}
		}

		log.Printf("Expected metric: %s", m)

		if m.GetCounter().GetValue() != 1 {
			return fmt.Errorf("expected grpc_server_msg_received_total to be 1, saw %+v instead", m)
		}

		return nil
	})
}

func hasLabels(labels []*dto.LabelPair, expected map[string]string) bool {
	for _, label := range labels {
		if val, ok := expected[label.GetName()]; ok {
			if val != label.GetValue() {
				return false
			}
			delete(expected, label.GetName())
		}
	}
	return len(expected) == 0
}
