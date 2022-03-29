// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"reflect"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fntypes"
	"namespacelabs.dev/foundation/workspace/cache"
)

func RegisterProtoCacheable() {
	RegisterCacheable[proto.Message](protoCacheable{})
}

type protoCacheable struct{}

func (protoCacheable) ComputeDigest(ctx context.Context, v interface{}) (fntypes.Digest, error) {
	_, h, err := marshalProto(v.(proto.Message))
	if err != nil {
		return fntypes.Digest{}, err
	}

	return h, nil
}

func (protoCacheable) LoadCached(ctx context.Context, c cache.Cache, t reflect.Type, h fntypes.Digest) (Result[proto.Message], error) {
	if t.Kind() != reflect.Ptr {
		return Result[proto.Message]{}, fnerrors.InternalError("expected %q to be a pointer", t.Name())
	}

	msg, ok := reflect.New(t.Elem()).Interface().(proto.Message)
	if !ok {
		return Result[proto.Message]{}, fnerrors.InternalError("expected %q to be a ptr to a proto.Message", t.Name())
	}

	bytes, err := c.Bytes(ctx, h)
	if err != nil {
		return Result[proto.Message]{}, err
	}

	if err := proto.Unmarshal(bytes, msg); err != nil {
		return Result[proto.Message]{}, err
	}

	return Result[proto.Message]{
		Digest: h,
		Value:  msg,
	}, nil
}

func (protoCacheable) Cache(ctx context.Context, c cache.Cache, msg proto.Message) (fntypes.Digest, error) {
	bytes, h, err := marshalProto(msg)
	if err != nil {
		return fntypes.Digest{}, err
	}

	if err := c.WriteBytes(ctx, h, bytes); err != nil {
		return fntypes.Digest{}, err
	}

	return h, err
}

func marshalProto(msg proto.Message) ([]byte, fntypes.Digest, error) {
	bytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(msg)
	if err != nil {
		return nil, fntypes.Digest{}, err
	}

	h, err := cache.DigestBytes(bytes)
	return bytes, h, err
}