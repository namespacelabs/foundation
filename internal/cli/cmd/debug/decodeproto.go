// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"

	"github.com/andybalholm/brotli"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

func newDecodeProtoCmd() *cobra.Command {
	var unbase64, unbrotli bool

	cmd := &cobra.Command{
		Use:   "decode-proto",
		Short: "Decodes a proto passed by stdin.",
		Args:  cobra.ExactArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			desc, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(args[0]))
			if err != nil {
				return err
			}

			input, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}

			if unbase64 {
				input, err = base64.RawStdEncoding.DecodeString(string(input))
				if err != nil {
					return err
				}
			}

			if unbrotli {
				input, err = ioutil.ReadAll(brotli.NewReader(bytes.NewReader(input)))
				if err != nil {
					return err
				}
			}

			m := desc.New().Interface()
			if err := proto.Unmarshal(input, m); err != nil {
				return err
			}

			out, err := prototext.MarshalOptions{Multiline: true}.Marshal(m)
			if err != nil {
				return err
			}

			_, _ = console.Stdout(ctx).Write(out)
			return nil
		}),
	}

	cmd.Flags().BoolVar(&unbase64, "unbase64", unbase64, "Assume incoming stream is base64 encoded.")
	cmd.Flags().BoolVar(&unbrotli, "unbrotli", unbrotli, "Assume incoming stream is brotli encoded.")

	return cmd
}
