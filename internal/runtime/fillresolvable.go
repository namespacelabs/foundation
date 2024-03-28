// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"context"
	"strings"
	"sync"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

type ResolvableSink interface {
	SetValue(key, value string) error
	SetSecret(key string, secret *SecretRef) error
	SetExperimentalFromDownwardsFieldPath(key, value string) error
	// XXX replace with late bound FieldSelector.
	SetLateBoundResourceFieldSelector(key string, _ runtimepb.SetContainerField_ValueSource, src *schema.ResourceConfigFieldSelector) error
}

type SecretRef struct {
	Name string
	Key  string
}

type ResolvableSecretSource interface {
	Allocate(context.Context, *schema.PackageRef) (*SecretRef, error)
}

type ResolvableSinkMap map[string]string

func (x *ResolvableSinkMap) SetValue(key, value string) error {
	(*x)[key] = value
	return nil
}

func (x *ResolvableSinkMap) SetSecret(key string, secret *SecretRef) error {
	return fnerrors.New("%s: secrets not supported in this context", key)
}

func (x *ResolvableSinkMap) SetExperimentalFromDownwardsFieldPath(key, value string) error {
	return fnerrors.New("%s: ExperimentalFromDownwardsFieldPath not supported in this context", key)
}

func (x *ResolvableSinkMap) SetLateBoundResourceFieldSelector(key string, _ runtimepb.SetContainerField_ValueSource, src *schema.ResourceConfigFieldSelector) error {
	return fnerrors.New("%s: late bound values not supported in this context", key)
}

type LikeResolvable interface {
	GetName() string
	GetValue() *schema.Resolvable
}

func ResolveResolvables[V LikeResolvable](ctx context.Context, rt *runtimepb.RuntimeConfig, secrets ResolvableSecretSource, resolvables []V, out ResolvableSink) error {
	pr := parallelResolver{out: out}

	eg := executor.New(ctx, "runtime.resolve-resolvables")
	for _, entry := range resolvables {
		entry := entry

		eg.Go(func(ctx context.Context) error {
			if entry.GetValue() == nil {
				return nil
			}

			return pr.resolve(ctx, rt, secrets, entry.GetName(), entry.GetValue())
		})
	}

	return eg.Wait()
}

type parallelResolver struct {
	mu  sync.Mutex
	out ResolvableSink
}

func (pr *parallelResolver) SetValue(key, value string) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.out.SetValue(key, value)
}

func (pr *parallelResolver) SetSecret(key string, secret *SecretRef) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.out.SetSecret(key, secret)
}

func (pr *parallelResolver) SetExperimentalFromDownwardsFieldPath(key, value string) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.out.SetExperimentalFromDownwardsFieldPath(key, value)
}

func (pr *parallelResolver) SetLateBoundResourceFieldSelector(key string, src runtimepb.SetContainerField_ValueSource, sel *schema.ResourceConfigFieldSelector) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.out.SetLateBoundResourceFieldSelector(key, src, sel)
}

func (pr *parallelResolver) resolve(ctx context.Context, rt *runtimepb.RuntimeConfig, secrets ResolvableSecretSource, fieldName string, resv *schema.Resolvable) error {
	switch {
	case resv.FromKubernetesSecret != "":
		parts := strings.SplitN(resv.FromKubernetesSecret, ":", 2)
		if len(parts) < 2 {
			return fnerrors.New("invalid from_kubernetes_secret format")
		}

		return pr.SetSecret(fieldName, &SecretRef{parts[0], parts[1]})

	case resv.ExperimentalFromDownwardsFieldPath != "":
		return pr.SetExperimentalFromDownwardsFieldPath(fieldName, resv.ExperimentalFromDownwardsFieldPath)

	case resv.FromSecretRef != nil:
		if secrets == nil {
			return fnerrors.InternalError("can't use FromSecretRef in this context")
		}

		alloc, err := secrets.Allocate(ctx, resv.FromSecretRef)
		if err != nil {
			return err
		}

		return pr.SetSecret(fieldName, alloc)

	case resv.FromServiceEndpoint != nil:
		endpoint, err := SelectServiceValue(rt, resv.FromServiceEndpoint, SelectServiceEndpoint)
		if err != nil {
			return err
		}
		return pr.SetValue(fieldName, endpoint)

	case resv.FromServiceIngress != nil:
		url, err := SelectServiceValue(rt, resv.FromServiceIngress, SelectServiceIngress)
		if err != nil {
			return err
		}
		return pr.SetValue(fieldName, url)

	case resv.FromResourceField != nil:
		return pr.SetLateBoundResourceFieldSelector(fieldName, runtimepb.SetContainerField_RESOURCE_CONFIG_FIELD_SELECTOR, resv.FromResourceField)

	case resv.FromFieldSelector != nil:
		instance, err := SelectInstance(rt, resv.FromFieldSelector.Instance)
		if err != nil {
			return err
		}

		x, err := resources.SelectField("fromFieldSelector", instance, resv.FromFieldSelector.FieldSelector)
		if err != nil {
			return err
		}

		vv, err := resources.CoerceAsString(x)
		if err != nil {
			return err
		}

		return pr.SetValue(fieldName, vv)

	default:
		return pr.SetValue(fieldName, resv.Value)
	}
}
