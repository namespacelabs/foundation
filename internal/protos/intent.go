// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package protos

import (
	"context"
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"reflect"

	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type ParseContext struct {
	SupportWellKnownMessages bool
	FS                       fs.FS
	PackageName              schema.PackageName
	EnsurePackage            func(schema.PackageName) error
}

func allocateValue(ctx context.Context, pctx ParseContext, parent protoreflect.Message, field protoreflect.FieldDescriptor, value any) (protoreflect.Value, error) {
	if field.IsMap() {
		return protoreflect.Value{}, fnerrors.Newf("maps not supported")
	}

	if field.IsList() {
		list := parent.NewField(field).Interface().(protoreflect.List)

		switch x := value.(type) {
		case []any:
			for k, v := range x {
				allocated, err := allocateSingleValue(ctx, pctx, field, v)
				if err != nil {
					return protoreflect.Value{}, fnerrors.Newf("[%d]: %w", k, err)
				}
				list.Append(allocated)
			}

			return protoreflect.ValueOfList(list), nil
		}

		return protoreflect.Value{}, fnerrors.Newf("expected []any, got %s", reflect.TypeOf(value).String())
	}

	return allocateSingleValue(ctx, pctx, field, value)
}

func allocateSingleValue(ctx context.Context, pctx ParseContext, field protoreflect.FieldDescriptor, value any) (protoreflect.Value, error) {
	switch field.Kind() {
	case protoreflect.BoolKind:
		switch x := value.(type) {
		case bool:
			return protoreflect.ValueOfBool(x), nil
		}

		return protoreflect.Value{}, fnerrors.Newf("expected bool, got %s", reflect.TypeOf(value).String())

	case protoreflect.DoubleKind,
		protoreflect.FloatKind:
		switch x := value.(type) {
		case float32, float64:
			return protoreflect.ValueOf(x), nil

		case json.Number:
			f, err := x.Float64()
			if err != nil {
				return protoreflect.Value{}, fnerrors.Newf("failed to parse json number %v as float: %w", x, err)
			}

			return protoreflect.ValueOf(f), nil
		}

		return protoreflect.Value{}, fnerrors.Newf("expected float, got %s", reflect.TypeOf(value).String())

	case protoreflect.Int32Kind,
		protoreflect.Fixed32Kind,
		protoreflect.Uint32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Sint32Kind:
		switch x := value.(type) {
		case int32, uint, uint32:
			return protoreflect.ValueOf(x), nil

		case json.Number:
			n, err := x.Int64()
			if err != nil {
				return protoreflect.Value{}, fnerrors.Newf("failed to parse json number %v as integer: %w", x, err)
			}

			return protoreflect.ValueOf(int32(n)), nil
		}

		return protoreflect.Value{}, fnerrors.Newf("expected int32 or uint32, got %s", reflect.TypeOf(value).String())

	case protoreflect.Int64Kind,
		protoreflect.Fixed64Kind,
		protoreflect.Uint64Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.Sint64Kind:
		switch x := value.(type) {
		case int32, int64, uint, uint32, uint64:
			return protoreflect.ValueOf(x), nil

		case json.Number:
			n, err := x.Int64()
			if err != nil {
				return protoreflect.Value{}, fnerrors.Newf("failed to parse json number %v as integer: %w", x, err)
			}

			return protoreflect.ValueOf(n), nil
		}

		return protoreflect.Value{}, fnerrors.Newf("expected int64 or uint64, got %s", reflect.TypeOf(value).String())

	case protoreflect.StringKind:
		switch x := value.(type) {
		case string:
			return protoreflect.ValueOf(x), nil
		}

		return protoreflect.Value{}, fnerrors.Newf("expected string, got %s", reflect.TypeOf(value).String())

	case protoreflect.BytesKind:
		switch x := value.(type) {
		case []byte:
			return protoreflect.ValueOf(x), nil
		}

		return protoreflect.Value{}, fnerrors.Newf("expected bytes, got %s", reflect.TypeOf(value).String())

	case protoreflect.EnumKind:
		switch x := value.(type) {
		case string:
			fieldValue := field.Enum().Values().ByName(protoreflect.Name(x))
			if fieldValue == nil {
				return protoreflect.Value{}, fnerrors.Newf("unknown enum value %s", x)
			}

			return protoreflect.ValueOfEnum(fieldValue.Number()), nil

		case int32:
			fieldValue := field.Enum().Values().ByNumber(protoreflect.EnumNumber(x))
			if fieldValue == nil {
				return protoreflect.Value{}, fnerrors.Newf("unknown enum value %v", x)
			}

			return protoreflect.ValueOfEnum(fieldValue.Number()), nil

		case json.Number:
			n, err := x.Int64()
			if err != nil {
				return protoreflect.Value{}, fnerrors.Newf("failed to parse json number %v as integer: %w", x, err)
			}

			fieldValue := field.Enum().Values().ByNumber(protoreflect.EnumNumber(n))
			if fieldValue == nil {
				return protoreflect.Value{}, fnerrors.Newf("unknown enum value %v", x)
			}

			return protoreflect.ValueOfEnum(fieldValue.Number()), nil
		}

		return protoreflect.Value{}, fnerrors.Newf("expected string or int32, got %s", reflect.TypeOf(value).String())

	case protoreflect.MessageKind:
		msg, err := AllocateWellKnownMessage(ctx, pctx, field.Message(), value)
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfMessage(msg.ProtoReflect()), nil

	default:
		return protoreflect.Value{}, fnerrors.Newf("kind not supported: %v", field.Kind())
	}
}

func AllocateWellKnownMessage(ctx context.Context, pctx ParseContext, messageType protoreflect.MessageDescriptor, value any) (protoreflect.ProtoMessage, error) {
	if pctx.SupportWellKnownMessages {
		// Handle well-known types.
		switch messageType.FullName() {
		case "foundation.schema.FileContents":
			return AllocateFileContents(ctx, pctx, value)

		case "foundation.schema.InlineJson":
			serialized, err := json.Marshal(value)
			if err != nil {
				return nil, fnerrors.Newf("value is not serializable as json: %w", err)
			}
			return &schema.InlineJson{InlineJson: string(serialized)}, nil

		case "foundation.schema.PackageRef":
			pkgref, err := allocatePackageRef(ctx, pctx, messageType, value)
			if err != nil {
				return nil, err
			}

			if ref, ok := pkgref.(*schema.PackageRef); ok {
				if pctx.EnsurePackage != nil {
					if err := pctx.EnsurePackage(ref.AsPackageName()); err != nil {
						return nil, err
					}
				}
			} else {
				return nil, fnerrors.InternalError("expected package ref parsing to yield a schema.PackageRef")
			}

			return pkgref, nil
		}
	}

	return AllocateDynamicMessage(ctx, pctx, messageType, value)
}

func AllocateDynamicMessage(ctx context.Context, pctx ParseContext, messageType protoreflect.MessageDescriptor, value any) (protoreflect.ProtoMessage, error) {
	msg := dynamicpb.NewMessage(messageType)
	return allocateMessage(ctx, pctx, msg, value)
}

func allocateMessage(ctx context.Context, pctx ParseContext, msg protoreflect.Message, value any) (protoreflect.ProtoMessage, error) {
	switch x := value.(type) {
	case map[string]interface{}:
		fields := msg.Descriptor().Fields()

		for key, v := range x {
			f := fields.ByJSONName(key)
			if f == nil {
				f = fields.ByName(protoreflect.Name(key))
			}

			if f == nil {
				return nil, fnerrors.Newf("{%s}.%q: no such field", msg.Descriptor().FullName(), key)
			}

			newValue, err := allocateValue(ctx, pctx, msg, f, v)
			if err != nil {
				return nil, fnerrors.Newf("{%s}.%q: %w", msg.Descriptor().FullName(), key, err)
			}

			msg.Set(f, newValue)
		}

		return msg.Interface(), nil
	}

	return nil, fnerrors.Newf("%s: expected map, got %s", msg.Descriptor().FullName(), reflect.TypeOf(value).String())
}

func allocatePackageRef(ctx context.Context, pctx ParseContext, messageType protoreflect.MessageDescriptor, value any) (protoreflect.ProtoMessage, error) {
	switch x := value.(type) {
	case string:
		if pctx.PackageName == "" {
			return nil, fnerrors.InternalError("failed to handle package ref, missing package name")
		}

		return schema.ParsePackageRef(pctx.PackageName, x)
	}

	return allocateMessage(ctx, pctx, (&schema.PackageRef{}).ProtoReflect(), value)
}

func AllocateFileContents(ctx context.Context, pctx ParseContext, value any) (*schema.FileContents, error) {
	if pctx.FS == nil {
		return nil, fnerrors.Newf("failed to handle resource, no workspace access available")
	}

	switch x := value.(type) {
	case string:
		contents, err := fs.ReadFile(pctx.FS, x)
		if err != nil {
			return nil, fnerrors.Newf("failed to load %q: %w", x, err)
		}

		return &schema.FileContents{
			Path:     x,
			Contents: contents,
		}, nil

	case map[string]interface{}:
		keys := maps.Keys(x)
		if len(keys) != 1 {
			return nil, fnerrors.Newf("failed to handle inline resource, expected single-key map")
		}

		switch keys[0] {
		case "inline":
			switch y := x[keys[0]].(type) {
			case string:
				return &schema.FileContents{
					Contents: []byte(y),
				}, nil

			case []byte:
				return &schema.FileContents{Contents: y}, nil
			}

			return nil, fnerrors.Newf("failed to handle inline resource, got %s", reflect.TypeOf(x[keys[0]]).String())

		case "fromURL":
			switch y := x[keys[0]].(type) {
			case map[string]any:
				if url, ok := str(y, "url"); ok {
					if rawDigest, ok := str(y, "digest"); ok {
						digest, err := schema.ParseDigest(rawDigest)
						if err != nil {
							return nil, err
						}

						d := download.URL(artifacts.Reference{
							URL:    url,
							Digest: digest,
						})

						res, err := compute.GetValue(ctx, d)
						if err != nil {
							return nil, err
						}

						r, err := res.Reader()
						if err != nil {
							return nil, err
						}

						defer r.Close()

						contents, err := ioutil.ReadAll(r)
						if err != nil {
							return nil, err
						}

						return &schema.FileContents{
							Contents: contents,
						}, nil
					}
				}

				return nil, fnerrors.Newf("failed to handle url resource, expected url and digest keys")
			}

			return nil, fnerrors.Newf("failed to handle url resource, got %s", reflect.TypeOf(x[keys[0]]).String())
		}

		return nil, fnerrors.Newf("failed to handle inline resource, expected %q got %q", "inline", keys[0])
	}

	return nil, fnerrors.Newf("failed to handle resource type, got %s", reflect.TypeOf(value).String())
}

func str(m map[string]any, key string) (string, bool) {
	if v, ok := m[key]; ok {
		str, ok := v.(string)
		return str, ok
	}
	return "", false
}
