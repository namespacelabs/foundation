// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/kr/text"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/std/tasks"
)

var ExplainIndentValues = false

type likeNamed interface {
	hasAction
	hasUnwrap
}

func Explain(ctx context.Context, w io.Writer, c rawComputable) error {
	var b bytes.Buffer
	if err := explain(ctx, &b, c, ""); err != nil {
		return err
	}
	_, err := b.WriteTo(w)
	return err
}

func explain(ctx context.Context, w io.Writer, c rawComputable, indent string) error {
	if named, ok := c.(likeNamed); ok {
		name, label := tasks.NameOf(named.Action())
		if label != "" {
			fmt.Fprintf(w, "[%s (%s)]", label, name)
		} else {
			fmt.Fprintf(w, "[%s]", name)
		}

		c = named.Unwrap()
	} else {
		fmt.Fprintf(w, "%s", typeStr(c))
	}

	opts := c.prepareCompute(c)

	fmt.Fprintf(w, " => %s ", typeStr(opts.OutputType))
	if opts.NonDeterministic {
		fmt.Fprintf(w, "🌀 ")
	}
	if opts.NotCacheable {
		fmt.Fprintf(w, "❗ ")
	}

	if e, ok := c.(interface {
		Explain(context.Context, io.Writer) error
	}); ok {
		fmt.Fprintf(w, "= ")
		return e.Explain(ctx, text.NewIndentWriter(w, []byte(indent+"  ")))
	}

	fmt.Fprintf(w, "= {\n")
	for _, in := range c.Inputs().ins {
		fmt.Fprintf(w, "  %s%s ", indent, in.Name)

		if in.Undetermined {
			fmt.Fprint(w, "⭕ ")
		}

		if child, ok := in.Value.(rawComputable); ok {
			if err := explain(ctx, w, child, indent+"  "); err != nil {
				return err
			}
		} else {
			if in.Value == nil {
				fmt.Fprintf(w, "(nil)")
			} else {
				fmt.Fprintf(w, "%s = ", typeStr(in.Value))

				switch x := in.Value.(type) {
				case proto.Message:
					fmt.Fprintf(w, "{ %s }", prototext.MarshalOptions{Multiline: ExplainIndentValues}.Format(x))
				case fmt.Stringer:
					fmt.Fprintf(w, "%s", x)
				default:
					if x == nil && in.Undetermined {
						// Do nothing.
					} else if serialized, err := json.MarshalIndent(x, indent+"  ", "  "); err == nil {
						fmt.Fprintf(w, "%s", serialized)
					} else {
						fmt.Fprint(w, "?")
					}
				}
			}
		}

		fmt.Fprintln(w)
	}

	for _, m := range c.Inputs().marshallers {
		var b bytes.Buffer
		var serialized string
		if err := m.Marshal(ctx, &b); err != nil {
			serialized = "?"
		} else {
			serialized = base64.RawStdEncoding.EncodeToString(b.Bytes())
		}

		fmt.Fprintf(w, "  %smarshalled %s = %s\n", indent, m.Name, serialized)
	}

	fmt.Fprintf(w, "%s}\n", indent)

	return nil
}
