// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/kr/text"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/codegen/protos/resolver"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func NewDescribeCmd() *cobra.Command {
	var (
		env  cfg.Context
		locs fncobra.Locations
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:  "describe <path/to/package:target>",
			Args: cobra.ExactArgs(1),
		}).
		With(
			fncobra.HardcodeEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{SupportPackageRef: true, RequireSingle: true})).
		DoWithArgs(func(ctx context.Context, args []string) error {
			style := colors.Ctx(ctx)
			stdout := console.Stdout(ctx)
			pl := parsing.NewPackageLoader(env)

			for _, loc := range locs.Locs {
				locs.Refs = append(locs.Refs, schema.MakePackageSingleRef(loc.AsPackageName()))
			}
			ref := locs.Refs[0]

			pkg, err := pl.LoadByName(ctx, ref.AsPackageName())
			if err != nil {
				return err
			}

			bodyWriter := indent(stdout)
			if ref.Name == "" {
				if pkg.Extension != nil {
					fmt.Fprintln(stdout, style.Comment.Apply("Extension"))
					formatProto(ctx, bodyWriter, pl, style, pkg.Extension)
					fmt.Fprintln(bodyWriter)
				}
				if pkg.Service != nil {
					fmt.Fprintln(stdout, style.Comment.Apply("Service"))
					formatProto(ctx, bodyWriter, pl, style, pkg.Service)
					fmt.Fprintln(bodyWriter)
				}
				if pkg.Server != nil {
					fmt.Fprintln(stdout, style.Comment.Apply("Server"))
					formatProto(ctx, bodyWriter, pl, style, pkg.Server)
					fmt.Fprintln(bodyWriter)
				}
				for _, rp := range pkg.ResourceProviders {
					formatResourceProvider(ctx, stdout, style, pl, &rp)
				}
			} else {
				binary, err := pkg.LookupBinary(ref.Name)
				if err == nil {
					fmt.Fprintf(stdout, "%s :%s\n", style.Comment.Apply("Binary"), binary.Name)
					formatProto(ctx, bodyWriter, pl, style, binary)
					fmt.Fprintln(bodyWriter)
				}

				for _, test := range pkg.Tests {
					if test.Name == ref.Name {
						fmt.Fprintf(stdout, "%s :%s\n", style.Comment.Apply("Test"), test.Name)
						formatProto(ctx, stdout, pl, style, test)
						fmt.Fprintln(bodyWriter)
					}
				}

				secret := pkg.LookupSecret(ref.Name)
				if secret != nil {
					fmt.Fprintf(stdout, "%s :%s\n", style.Comment.Apply("Secret"), secret.Name)
					formatProto(ctx, stdout, pl, style, secret)
					fmt.Fprintln(bodyWriter)
				}

				for _, vol := range pkg.Volumes {
					if vol.Name == ref.Name {
						formatVolume(ctx, stdout, style, pl, vol)
					}
				}

				resClass := pkg.LookupResourceClass(ref.Name)
				if resClass != nil {
					formatResourceClass(stdout, style, resClass)
				}

				res := pkg.LookupResourceInstance(ref.Name)
				if res != nil {
					formatResourceInstance(ctx, stdout, style, pl, res)
				}
			}

			return nil
		})
}

func formatResourceClass(w io.Writer, style colors.Style, resClass *pkggraph.ResourceClass) {
	fmt.Fprintf(w, "%s :%s\n", style.Comment.Apply("ResourceClass"), resClass.Ref.Name)
	resout := indent(w)

	fmt.Fprintf(resout, "intent: ")
	formatMessageDef(resout, resClass.IntentType.Descriptor, style)
	fmt.Fprintln(resout)

	fmt.Fprintf(resout, "output: ")
	formatMessageDef(resout, resClass.InstanceType.Descriptor, style)
	fmt.Fprintln(resout)

	if resClass.DefaultProvider != "" {
		fmt.Fprintf(resout, "default_provider: %s\n", resClass.DefaultProvider)
	} else {
		fmt.Fprintf(resout, "default_provider: %s\n", style.LessRelevant.Apply("none"))
	}

	// TODO: Print all known providers in the dep tree.
}

func formatResourceInstance(ctx context.Context, w io.Writer, style colors.Style, pl *parsing.PackageLoader, res *pkggraph.ResourceInstance) {
	fmt.Fprintf(w, "%s :%s\n", style.Comment.Apply("Resource"), res.ResourceRef.Name)
	resout := indent(w)

	fmt.Fprintf(resout, "Class: %s\n", res.Spec.Class.Ref.Canonical())
	fmt.Fprintf(resout, "Provider: %s\n", res.Spec.Provider.Spec.PackageName)
	fmt.Fprintf(resout, "Intent: ")
	formatProto(ctx, resout, pl, style, res.Spec.Intent)
	fmt.Fprintln(resout)
	fmt.Fprint(resout, "ResouceInputs: ")
	if len(res.Spec.ResourceInputs) == 0 {
		fmt.Fprintln(resout, style.LessRelevant.Apply("none"))
	} else {
		fmt.Fprintln(resout)
		for _, inp := range res.Spec.ResourceInputs {
			fmt.Fprintf(resout, "    %s\n", inp.ResourceID)
		}
	}
}

func formatResourceProvider(ctx context.Context, w io.Writer, style colors.Style, pl *parsing.PackageLoader, rp *pkggraph.ResourceProvider) {

	fmt.Fprintf(w, "%s %s\n",
		style.Comment.Apply("ResourceProvider"), formatPkgRef(rp.Spec.ProvidesClass))
	iw := indent(w)
	fmt.Fprint(iw, "IntentType: ")
	if rp.IntentType != nil {
		fmt.Fprintln(iw, rp.Spec.IntentType.ProtoType)
	} else {
		fmt.Fprintln(iw, style.LessRelevant.Apply("none"))
	}

	if rp.Spec.InitializedWith != nil {
		fmt.Fprint(iw, "InitializedWith: ")
		formatProto(ctx, iw, pl, style, rp.Spec.InitializedWith)
		fmt.Fprintln(iw)
	}

	if rp.Spec.PrepareWith != nil {
		fmt.Fprint(iw, "PrepareWith: ")
		formatProto(ctx, iw, pl, style, rp.Spec.PrepareWith)
		fmt.Fprintln(iw)
	}
}

func formatVolume(ctx context.Context, w io.Writer, style colors.Style, pl *parsing.PackageLoader, vol *schema.Volume) {
	fmt.Fprintf(w, "%s :%s %s\n",
		style.Comment.Apply("Volume"), vol.Name,
		style.LessRelevant.Apply(fmt.Sprintf("(%s)", vol.Kind)))

	iw := indent(w)
	formatProto(ctx, iw, pl, style, vol.Definition)
	fmt.Fprintln(iw)
}

// Formats a MessageDescriptor as a CUE type, correctly handling Namespace well-known types.
func formatMessageDef(w io.Writer, desc protoreflect.MessageDescriptor, style colors.Style) {
	if desc.FullName() == "foundation.schema.FileContents" {
		fmt.Fprintf(w, `"%s" | { inline: "%s" }`,
			style.LessRelevant.Apply("path/to/file"),
			style.LessRelevant.Apply("inline content"))
		return
	} else if desc.FullName() == "foundation.schema.PackageRef" {
		fmt.Fprintf(w, `"%s"`, style.LessRelevant.Apply("path/to/package"))
		return
	}

	fmt.Fprint(w, "{\n")
	cw := indent(w)
	src := desc.ParentFile().SourceLocations()
	fields := desc.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		loc := src.ByDescriptor(field)

		if loc.LeadingComments != "" {
			printComments(cw, loc.LeadingComments, style)
			fmt.Fprintln(cw)
		}
		fmt.Fprintf(cw, "%s: ", field.JSONName())
		if field.IsList() {
			fmt.Fprint(cw, "[")
		}
		switch field.Kind() {
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Uint32Kind,
			protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Uint64Kind,
			protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind, protoreflect.Fixed64Kind, protoreflect.Fixed32Kind:
			fmt.Fprint(cw, style.LessRelevant.Apply("int"))
		case protoreflect.FloatKind, protoreflect.DoubleKind:
			fmt.Fprint(cw, style.LessRelevant.Apply("number"))
		case protoreflect.BoolKind:
			fmt.Fprint(cw, style.LessRelevant.Apply("bool"))
		case protoreflect.StringKind:
			fmt.Fprint(cw, style.LessRelevant.Apply("string"))
		case protoreflect.BytesKind:
			fmt.Fprint(cw, style.LessRelevant.Apply("bytes"))
		case protoreflect.EnumKind:
			opts := field.Enum().Values()
			for j := 0; j < opts.Len(); j++ {
				opt := opts.Get(j)
				if j > 0 {
					fmt.Fprint(cw, style.LessRelevant.Apply(" | "))
				}
				fmt.Fprintf(cw, "%q", style.LessRelevant.Apply(string(opt.Name())))
			}
		case protoreflect.MessageKind, protoreflect.GroupKind:
			formatMessageDef(cw, field.Message(), style)
		}
		if field.IsList() {
			fmt.Fprint(cw, "]")
		}
		if loc.TrailingComments != "" {
			fmt.Fprint(cw, " ")
			printComments(cw, loc.TrailingComments, style)
		}
		fmt.Fprintln(cw)
	}
	fmt.Fprint(w, "}")
}

func printComments(w io.Writer, s string, style colors.Style) {
	lines := strings.Split(strings.Trim(s, "\n"), "\n")
	for i, l := range lines {
		fmt.Fprint(w, style.Comment.Apply("//"), style.Comment.Apply(l))
		if i+1 < len(lines) {
			fmt.Fprintln(w)
		}
	}
}

func formatProto(ctx context.Context, w io.Writer, pl *parsing.PackageLoader, style colors.Style, msg protoreflect.ProtoMessage) {
	res := resolver.NewResolver(ctx, pl)

	if any, ok := msg.(*anypb.Any); ok {
		var err error
		msg, err = anypb.UnmarshalNew(any, proto.UnmarshalOptions{Resolver: res})
		if err != nil {
			fmt.Print(w, style.LessRelevant.Apply(fmt.Sprintf("error %v", err)))
		}
	}

	body, err := (protojson.MarshalOptions{
		UseProtoNames: true,
		Indent:        "    ",
		Resolver:      res,
	}).Marshal(msg)
	if err != nil {
		fmt.Print(w, style.LessRelevant.Apply(fmt.Sprintf("error %v", err)))
	}
	w.Write(body)
}

func indent(w io.Writer) io.Writer {
	return text.NewIndentWriter(w, []byte("    "))
}
