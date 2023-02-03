// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func canonicalizeJsonPath(loc pkggraph.Location, originalDesc, desc protoreflect.MessageDescriptor, originalSel, fieldSel string) (string, error) {
	parts := strings.SplitN(fieldSel, ".", 2)

	f := desc.Fields().ByTextName(parts[0])
	if f == nil {
		f = desc.Fields().ByJSONName(parts[0])
	}

	if f == nil {
		return "", fnerrors.NewWithLocation(loc, "%s: %q is not a valid field selector (%q doesn't match anything)", originalDesc.FullName(), originalSel, parts[0])
	}

	if len(parts) == 1 {
		if isSupportedProtoPrimitive(f) {
			return string(f.Name()), nil
		} else {
			return "", fnerrors.NewWithLocation(loc, "%s: %q is not a valid field selector (%q picks unsupported %v)", originalDesc.FullName(), originalSel, parts[0], f.Kind())
		}
	}

	if f.Kind() != protoreflect.MessageKind {
		var hint string
		if isSupportedProtoPrimitive(f) {
			hint = ": cannot select fields inside primitive types"
		}

		return "", fnerrors.NewWithLocation(loc, "%s: %q is not a valid field selector (%q picks unsupported %v)%s", originalDesc.FullName(), originalSel, parts[0], f.Kind(), hint)
	}

	selector, err := canonicalizeJsonPath(loc, originalDesc, f.Message(), originalSel, parts[1])
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s", f.Name(), selector), nil
}

func isSupportedProtoPrimitive(f protoreflect.FieldDescriptor) bool {
	switch f.Kind() {
	case protoreflect.StringKind, protoreflect.Int32Kind, protoreflect.Uint32Kind, protoreflect.Int64Kind, protoreflect.Uint64Kind:
		return true

	default:
		return false
	}
}
