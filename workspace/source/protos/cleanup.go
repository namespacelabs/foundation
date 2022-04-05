// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	p "namespacelabs.dev/foundation/std/proto"
)

func CleanupForNonProvisioning(msg protoreflect.Message) {
	cleanWith(msg, p.E_ProvisionOnly)
}

func CleanupSensitive(msg protoreflect.Message) {
	cleanWith(msg, p.E_IsSensitive)
}

func cleanWith(msg protoreflect.Message, xt protoreflect.ExtensionType) {
	fields := msg.Descriptor().Fields()
	for k := 0; k < fields.Len(); k++ {
		field := fields.Get(k)

		opts := field.Options().(*descriptorpb.FieldOptions)
		if opts != nil {
			x := proto.GetExtension(opts, xt)
			if b := x.(bool); b {
				msg.Clear(field)
				continue
			}
		}

		if field.Kind() == protoreflect.MessageKind {
			cleanWith(msg.Get(field).Message(), xt)
		}
	}
}
