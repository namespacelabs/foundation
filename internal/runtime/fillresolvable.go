// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"context"
	"strings"

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
	allocated := make([]*SecretRef, len(resolvables))

	eg := executor.New(ctx, "runtime.preallocate-secrets")
	for k, entry := range resolvables {
		if entry.GetValue().GetFromSecretRef() == nil {
			continue
		}

		if secrets == nil {
			return fnerrors.InternalError("can't use FromSecretRef in this context")
		}

		k := k
		entry := entry

		eg.Go(func(ctx context.Context) error {
			alloc, err := secrets.Allocate(ctx, entry.GetValue().GetFromSecretRef())
			if err != nil {
				return err
			}

			allocated[k] = alloc
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	for k, entry := range resolvables {
		if entry.GetValue() == nil {
			continue
		}

		if err := resolve(rt, allocated[k], entry.GetName(), entry.GetValue(), out); err != nil {
			return err
		}
	}

	return nil
}

func resolve(rt *runtimepb.RuntimeConfig, alloc *SecretRef, fieldName string, resv *schema.Resolvable, out ResolvableSink) error {
	switch {
	case resv.FromKubernetesSecret != "":
		parts := strings.SplitN(resv.FromKubernetesSecret, ":", 2)
		if len(parts) < 2 {
			return fnerrors.New("invalid from_kubernetes_secret format")
		}

		return out.SetSecret(fieldName, &SecretRef{parts[0], parts[1]})

	case resv.ExperimentalFromDownwardsFieldPath != "":
		return out.SetExperimentalFromDownwardsFieldPath(fieldName, resv.ExperimentalFromDownwardsFieldPath)

	case resv.FromSecretRef != nil:
		if alloc == nil {
			return fnerrors.InternalError("secret %s was not allocated", resv.FromSecretRef.Canonical())
		}

		return out.SetSecret(fieldName, alloc)

	case resv.FromServiceEndpoint != nil:
		endpoint, err := SelectServiceValue(rt, resv.FromServiceEndpoint, SelectServiceEndpoint)
		if err != nil {
			return err
		}
		return out.SetValue(fieldName, endpoint)

	case resv.FromServiceIngress != nil:
		url, err := SelectServiceValue(rt, resv.FromServiceIngress, SelectServiceIngress)
		if err != nil {
			return err
		}
		return out.SetValue(fieldName, url)

	case resv.FromResourceField != nil:
		return out.SetLateBoundResourceFieldSelector(fieldName, runtimepb.SetContainerField_RESOURCE_CONFIG_FIELD_SELECTOR, resv.FromResourceField)

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

		return out.SetValue(fieldName, vv)

	default:
		return out.SetValue(fieldName, resv.Value)
	}
}
