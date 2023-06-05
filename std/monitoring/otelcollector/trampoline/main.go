// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"log"
	"os"
	"syscall"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"sigs.k8s.io/yaml"
)

type obj map[string]any

func main() {
	exporters := obj{
		"otlp/inclusterjaeger": obj{
			"endpoint": os.Getenv("JAEGER_ENDPOINT"),
			"tls":      obj{"insecure": true},
		},
	}

	if honeyCombTeam := os.Getenv("HONEYCOMB_TEAM"); honeyCombTeam != "" {
		exporters["otlp/honeycomb"] = obj{
			"endpoint": "api.honeycomb.io:443",
			"headers": obj{
				"x-honeycomb-team": honeyCombTeam,
			},
		}
	}

	config := obj{
		"receivers": obj{
			"otlp": obj{
				"protocols": obj{
					"grpc": obj{},
					"http": obj{},
				},
			},
		},
		"processors": obj{
			"batch": obj{},
		},
		"exporters": exporters,
		"service": obj{
			"pipelines": obj{
				"traces/1": obj{
					"receivers":  []string{"otlp"},
					"processors": []string{"batch"},
					"exporters":  sorted(maps.Keys(exporters)),
				},
			},
		},
	}

	bytes, err := yaml.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}

	filename, err := writeTemp(bytes)
	if err != nil {
		log.Fatal(err)
	}

	if err := syscall.Exec("/otelcol-contrib", []string{"otelcol", "--config=" + filename}, os.Environ()); err != nil {
		log.Fatal(err)
	}
}

func writeTemp(contents []byte) (string, error) {
	t, err := os.Create("/otel/conf/generated.yaml")
	if err != nil {
		return "", err
	}

	if _, err := t.Write(contents); err != nil {
		return "", err
	}

	return t.Name(), t.Close()
}

func sorted(strs []string) []string {
	slices.Sort(strs)
	return strs
}
