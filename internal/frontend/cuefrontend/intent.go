// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"io/fs"
	"reflect"
	"unicode/utf8"

	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func allocateValue(fsys fs.FS, parent *dynamicpb.Message, field protoreflect.FieldDescriptor, value any) (protoreflect.Value, error) {
	if field.IsMap() {
		return protoreflect.Value{}, fnerrors.New("maps not supported")
	}

	if field.IsList() {
		list := parent.NewField(field).Interface().(protoreflect.List)

		switch x := value.(type) {
		case []any:
			for k, v := range x {
				allocated, err := allocateSingleValue(fsys, field, v)
				if err != nil {
					return protoreflect.Value{}, fnerrors.New("[%d]: %w", k, err)
				}
				list.Append(allocated)
			}

			return protoreflect.ValueOfList(list), nil
		}

		return protoreflect.Value{}, fnerrors.New("expected []any, got %s", reflect.TypeOf(value).String())
	}

	return allocateSingleValue(fsys, field, value)
}

func allocateSingleValue(fsys fs.FS, field protoreflect.FieldDescriptor, value any) (protoreflect.Value, error) {
	switch field.Kind() {
	case protoreflect.BoolKind:
		switch x := value.(type) {
		case bool:
			return protoreflect.ValueOfBool(x), nil
		}

		return protoreflect.Value{}, fnerrors.New("expected bool, got %s", reflect.TypeOf(value).String())

	case protoreflect.DoubleKind,
		protoreflect.FloatKind:
		switch x := value.(type) {
		case float32, float64:
			return protoreflect.ValueOf(x), nil
		}

		return protoreflect.Value{}, fnerrors.New("expected float, got %s", reflect.TypeOf(value).String())

	case protoreflect.Int32Kind,
		protoreflect.Fixed32Kind,
		protoreflect.Uint32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Int64Kind,
		protoreflect.Fixed64Kind,
		protoreflect.Uint64Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.Sint64Kind:
		switch x := value.(type) {
		case int32, int64, uint, uint32, uint64:
			return protoreflect.ValueOf(x), nil
		}

		return protoreflect.Value{}, fnerrors.New("expected int{32,64} or uint{32,64}, got %s", reflect.TypeOf(value).String())

	case protoreflect.StringKind:
		switch x := value.(type) {
		case string:
			return protoreflect.ValueOf(x), nil
		}

		return protoreflect.Value{}, fnerrors.New("expected string, got %s", reflect.TypeOf(value).String())

	case protoreflect.BytesKind:
		switch x := value.(type) {
		case []byte:
			return protoreflect.ValueOf(x), nil
		}

		return protoreflect.Value{}, fnerrors.New("expected bytes, got %s", reflect.TypeOf(value).String())

	case protoreflect.EnumKind:
		switch x := value.(type) {
		case string:
			fieldValue := field.Enum().Values().ByName(protoreflect.Name(x))
			if fieldValue == nil {
				return protoreflect.Value{}, fnerrors.New("unknown enum value %s", x)
			}

			return protoreflect.ValueOfEnum(fieldValue.Number()), nil

		case int32:
			fieldValue := field.Enum().Values().ByNumber(protoreflect.EnumNumber(x))
			if fieldValue == nil {
				return protoreflect.Value{}, fnerrors.New("unknown enum value %v", x)
			}

			return protoreflect.ValueOfEnum(fieldValue.Number()), nil
		}

		return protoreflect.Value{}, fnerrors.New("expected string or int32, got %s", reflect.TypeOf(value).String())

	case protoreflect.MessageKind:
		msg, err := allocateMessage(fsys, field.Message(), value)
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfMessage(msg.ProtoReflect()), nil

	default:
		return protoreflect.Value{}, fnerrors.New("kind not supported: %v", field.Kind())
	}
}

func allocateMessage(fsys fs.FS, messageType protoreflect.MessageDescriptor, value any) (protoreflect.ProtoMessage, error) {
	// Handle well-known types.
	switch messageType.FullName() {
	case "foundation.schema.FileContents":
		return allocateFileContents(fsys, value)
	}

	switch x := value.(type) {
	case map[string]interface{}:
		msg := dynamicpb.NewMessage(messageType)
		fields := msg.Descriptor().Fields()

		for key, v := range x {
			f := fields.ByJSONName(key)
			if f == nil {
				f = fields.ByName(protoreflect.Name(key))
			}

			if f == nil {
				return nil, fnerrors.New("{%s}.%q: no such field", msg.Descriptor().FullName(), key)
			}

			newValue, err := allocateValue(fsys, msg, f, v)
			if err != nil {
				return nil, fnerrors.New("{%s}.%q: %w", msg.Descriptor().FullName(), key, err)
			}

			msg.Set(f, newValue)
		}

		return msg, nil
	}

	return nil, fnerrors.New("expected map, got %s", reflect.TypeOf(value).String())
}

func allocateFileContents(fsys fs.FS, value any) (protoreflect.ProtoMessage, error) {
	if fsys == nil {
		return nil, fnerrors.New("failed to handle resource, no workspace access available")
	}

	switch x := value.(type) {
	case string:
		contents, err := fs.ReadFile(fsys, x)
		if err != nil {
			return nil, fnerrors.New("failed to load %q: %w", x, err)
		}

		return &schema.FileContents{
			Utf8:     utf8.Valid(contents),
			Contents: contents,
		}, nil

	case map[string]interface{}:
		keys := maps.Keys(x)
		if len(keys) != 1 {
			return nil, fnerrors.New("failed to handle inline resource, expected single-key map")
		}

		switch keys[0] {
		case "inline":
			switch y := x[keys[0]].(type) {
			case string:
				return &schema.FileContents{
					Utf8:     true,
					Contents: []byte(y),
				}, nil

			case []byte:
				return &schema.FileContents{Contents: y}, nil
			}

			return nil, fnerrors.New("failed to handle inline resource, got %s", reflect.TypeOf(x[keys[0]]).String())
		}

		return nil, fnerrors.New("failed to handle inline resource, expected %q got %q", "inline", keys[0])
	}

	return nil, fnerrors.New("failed to handle resource type, got %s", reflect.TypeOf(value).String())
}
