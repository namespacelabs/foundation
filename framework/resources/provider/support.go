// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package provider

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"reflect"

	"namespacelabs.dev/foundation/framework/resources"
)

const ProtocolVersion = "1"

type Message struct {
	Version                string  `json:"version"`
	SerializedInstanceJSON *string `json:"serialized_instance,omitempty"`
	// TODO add orchestration event.
}

type ProviderContext struct {
	ProtocolVersion string `json:"protocol_version"`
}

type Provider[T any] struct {
	Context   ProviderContext
	Intent    T
	Resources *resources.Parsed
}

func MustPrepare[T any]() (context.Context, *Provider[T]) {
	intentFlag := flag.String("intent", "", "The serialized JSON intent.")
	resourcesFlag := flag.String("resources", "", "The serialized JSON resources.")
	providerCtxFlag := flag.String("provider_context", "", "The serialized JSON of a provider request.")

	flag.Parse()

	var pctx ProviderContext
	if *providerCtxFlag != "" {
		if err := json.Unmarshal([]byte(*providerCtxFlag), &pctx); err != nil {
			log.Fatalf("failed to parse provider context: %v", err)
		}
	} else {
		pctx.ProtocolVersion = "0"
	}

	var intentT T
	intent := reflect.New(reflect.TypeOf(intentT).Elem()).Interface().(T)

	resources, err := prepare(*intentFlag, *resourcesFlag, intent)
	if err != nil {
		log.Fatal(err.Error())
	}

	return context.Background(), &Provider[T]{Context: pctx, Intent: intent, Resources: resources}
}

func (p *Provider[T]) EmitResult(instance any) {
	serialized, err := json.Marshal(instance)
	if err != nil {
		log.Fatalf("failed to marshal instance: %v", err)
	}

	switch p.Context.ProtocolVersion {
	case "0":
		// Backwards compatibility.
		fmt.Printf("namespace.provision.result: %s\n", serialized)

	default:
		str := string(serialized)
		p.emitMessage(Message{SerializedInstanceJSON: &str})
	}
}

func (p *Provider[T]) emitMessage(message Message) {
	message.Version = ProtocolVersion

	serialized, err := json.Marshal(message)
	if err != nil {
		log.Fatalf("failed to marshal instance: %v", err)
	}

	fmt.Printf("namespace.provision.message: %s\n", serialized)
}

func prepare(intentFlag, resourcesFlag string, intent any) (*resources.Parsed, error) {
	if intentFlag == "" {
		return nil, fmt.Errorf("--intent is required")
	}

	if err := json.Unmarshal([]byte(intentFlag), intent); err != nil {
		return nil, fmt.Errorf("failed to decode intent: %w", err)
	}

	if resourcesFlag == "" {
		return nil, fmt.Errorf("--resources is required")
	}

	return resources.ParseResourceData([]byte(resourcesFlag))
}
