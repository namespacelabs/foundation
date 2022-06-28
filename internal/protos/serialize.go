// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"context"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
)

type TextAndBinary struct {
	Text   []byte
	JSON   []byte
	Binary []byte
}

type SerializeOpts struct {
	TextProto bool
	JSON      bool
	Resolver  interface {
		protoregistry.ExtensionTypeResolver
		protoregistry.MessageTypeResolver
	}
}

func SerializeMultiple(msgs ...proto.Message) ([]TextAndBinary, error) {
	return SerializeOpts{}.Serialize(msgs...)
}

func (opts SerializeOpts) Serialize(msgs ...proto.Message) ([]TextAndBinary, error) {
	var res []TextAndBinary
	for _, m := range msgs {
		binary, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
		if err != nil {
			return nil, fnerrors.New("proto serialized failed: %w", err)
		}

		tb := TextAndBinary{Binary: binary}

		if opts.TextProto {
			text, err := prototext.MarshalOptions{Multiline: true, Resolver: opts.Resolver}.Marshal(m)
			if err != nil {
				return nil, fnerrors.New("textproto serialized failed: %w", err)
			}
			tb.Text = text
		}

		if opts.JSON {
			json, err := protojson.MarshalOptions{Multiline: true, Resolver: opts.Resolver}.Marshal(m)
			if err != nil {
				return nil, fnerrors.New("json serialized failed: %w", err)
			}
			tb.JSON = json
		}

		res = append(res, tb)
	}

	return res, nil
}

func (opts SerializeOpts) SerializeToFS(ctx context.Context, target fnfs.WriteFS, m map[string]proto.Message) error {
	var files []fnfs.File
	for base, msg := range m {
		tb, err := opts.Serialize(msg)
		if err != nil {
			return err
		}

		for _, tb := range tb {
			files = append(files, fnfs.File{Path: base + ".binarypb", Contents: tb.Binary})
			if tb.JSON != nil {
				files = append(files, fnfs.File{Path: base + ".json", Contents: tb.JSON})
			}
			if tb.Text != nil {
				files = append(files, fnfs.File{Path: base + ".textpb", Contents: tb.Text})
			}
		}
	}

	for _, f := range files {
		if err := fnfs.WriteFile(ctx, target, f.Path, f.Contents, 0644); err != nil {
			return err
		}
	}

	return nil
}
