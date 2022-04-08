// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package codegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue/format"
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/schema"
	p "namespacelabs.dev/foundation/std/proto"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

const wellKnownResource = "foundation.std.types.Resource"

func Register() {
	ops.Register[*OpGenNode](generator{})
}

type generator struct{}

func (generator) Run(ctx context.Context, env ops.Environment, _ *schema.Definition, msg *OpGenNode) (*ops.DispatcherResult, error) {
	wenv, ok := env.(workspace.Packages)
	if !ok {
		return nil, errors.New("workspace.Packages required")
	}

	loc, err := wenv.Resolve(ctx, schema.PackageName(msg.Node.PackageName))
	if err != nil {
		return nil, err
	}

	return nil, generateNode(ctx, loc, msg.Node, msg.Protos, loc.Module.ReadWriteFS())
}

func ForNode(pkg *workspace.Package, available []*schema.Node) ([]*schema.Definition, error) {
	var allDefs []*schema.Definition

	if len(pkg.Provides) > 0 {
		var dl defs.DefList

		var list []*protos.FileDescriptorSetAndDeps
		for _, dl := range pkg.Provides {
			list = append(list, dl)
		}

		dl.Add("Generate Foundation exports", &OpGenNode{
			Node:   pkg.Node(),
			Protos: protos.Merge(list...),
		}, pkg.PackageName())

		if lst, err := dl.Serialize(); err != nil {
			return nil, err
		} else {
			allDefs = append(allDefs, lst...)
		}
	}

	for _, fmwk := range pkg.Node().CodegeneratedFrameworks() {
		defs, err := languages.IntegrationFor(fmwk).GenerateNode(pkg, available)
		if err != nil {
			return nil, err
		}

		allDefs = append(allDefs, defs...)
	}

	return allDefs, nil
}

func generateNode(ctx context.Context, loc workspace.Location, n *schema.Node, parsed *protos.FileDescriptorSetAndDeps, fs fnfs.ReadWriteFS) error {
	var imports uniquestrings.List

	pd, err := protodesc.NewFiles(parsed.AsFileDescriptorSet())
	if err != nil {
		return err
	}

	resolver, err := protos.AsResolver(pd)
	if err != nil {
		return err
	}

	types := make([]protoreflect.MessageType, len(n.Provides))
	for k, p := range n.Provides {
		msg, err := resolver.FindMessageByName(protoreflect.FullName(p.GetType().GetTypename()))
		if err != nil {
			return fnerrors.InternalError("%s: type not found: %w", p.GetType().GetTypename(), err)
		}

		types[k] = msg
	}

	var out bytes.Buffer

	fmt.Fprintf(&out, "#Exports: {\n")
	for k, p := range n.Provides {
		fmt.Fprintf(&out, "%s: {\n", p.Name)

		if t := types[k]; t.Descriptor().Fields().Len() > 0 {
			if err := generateProto(&out, resolver, t.Descriptor(), &imports); err != nil {
				return err
			}
			fmt.Fprintln(&out)
		}

		fmt.Fprintf(&out, "#Definition: {\n")
		fmt.Fprintf(&out, "packageName: %q\n", n.PackageName)
		fmt.Fprintf(&out, "type: %q\n", p.Name)
		fmt.Fprintf(&out, "typeDefinition: ")
		enc := json.NewEncoder(&out)
		enc.SetIndent("", "  ")
		enc.Encode(p.Type)
		fmt.Fprintf(&out, "}\n")

		fmt.Fprintln(&out, "}")
	}
	fmt.Fprintf(&out, "}\n")

	var final bytes.Buffer
	fmt.Fprintf(&final, "// This file is automatically generated.\npackage %s\n\n", packageName(loc))

	importStrs := imports.Strings()

	if len(importStrs) > 0 {
		sort.Strings(importStrs)
		fmt.Fprintf(&final, "import (\n")
		for _, imp := range importStrs {
			fmt.Fprintf(&final, "%q\n", imp)
		}
		fmt.Fprintf(&final, ")\n")
	}

	out.WriteTo(&final)

	return fnfs.WriteWorkspaceFile(ctx, fs, loc.Rel("exports.fn.cue"), func(w io.Writer) error {
		formatted, err := format.Source(final.Bytes())
		if err != nil {
			return err
		}

		_, err = w.Write(formatted)
		return err
	})
}

func generateProto(out io.Writer, parsed protos.AnyResolver, msg protoreflect.MessageDescriptor, imports *uniquestrings.List) error {
	var missingDef int
	for k := 0; k < msg.Fields().Len(); k++ {
		field := msg.Fields().Get(k)
		var t string
		switch field.Kind() {
		case protoreflect.BoolKind:
			t = "bool"
		case protoreflect.DoubleKind,
			protoreflect.FloatKind:
			t = "float"
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
			t = "int"
		case protoreflect.StringKind:
			t = "string"
			opts := field.Options().(*descriptorpb.FieldOptions)
			if opts != nil {
				x, err := proto.GetExtension(opts, p.E_IsPackage)
				if err == nil {
					if b := x.(*bool); b != nil && *b {
						t = "inputs.#Package"
						imports.Add("namespacelabs.dev/foundation/std/fn:inputs")
					}
				} else if err != proto.ErrMissingExtension {
					return err
				}
			}
		case protoreflect.BytesKind:
			t = "bytes"
		case protoreflect.EnumKind:
			var values []string
			enum := field.Enum()
			for j := 0; j < enum.Values().Len(); j++ {
				v := enum.Values().Get(j)
				values = append(values, fmt.Sprintf("%q", v.Name()))
			}
			t = fmt.Sprintf("(%s)", strings.Join(values, "|"))
		case protoreflect.MessageKind:
			msg := field.Message()
			if msg.FullName() == wellKnownResource {
				t = "types.#Resource"
				imports.Add("namespacelabs.dev/foundation/std/fn:types")
			} else if msg.FullName() == "google.protobuf.Any" {
				t = "types.#Any"
				imports.Add("namespacelabs.dev/foundation/std/fn:types")
			} else {
				var b bytes.Buffer
				fmt.Fprintln(&b, "{")
				if err := generateProto(&b, parsed, msg, imports); err != nil {
					return err
				}
				fmt.Fprint(&b, "}")
				t = b.String()
			}
		}

		if t == "" {
			missingDef++
		} else {
			if field.Cardinality() == protoreflect.Repeated {
				t = "[..." + t + "]"
			}
			fmt.Fprintf(out, "%s?: %s\n", field.JSONName(), t)
		}
	}

	if missingDef > 0 {
		fmt.Fprintf(out, "...\n")
	}

	return nil
}

func packageName(loc workspace.Location) string {
	if loc.Rel() == "." {
		return filepath.Base(loc.Module.ModuleName())
	}

	return filepath.Base(loc.Rel())
}
