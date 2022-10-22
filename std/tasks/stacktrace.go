// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tasks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/kr/text"
	"helm.sh/helm/v3/pkg/time"
	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
)

var OutputFullTraceCallsToDebug bool = false

func TraceCaller(ctx context.Context, makeLogger func(context.Context) io.Writer, name string) {
	trace := stacktrace.New()
	var b bytes.Buffer
	fmt.Fprintf(&b, "%+v", trace)

	actualName := fmt.Sprintf("%s-%v", name, time.Now().UnixNano())

	clean := strings.TrimSpace(b.String())
	_ = Attachments(ctx).AttachSerializable(actualName, "", clean)

	w := makeLogger(ctx)
	fmt.Fprintf(w, "Trace: %s\n", name)

	if OutputFullTraceCallsToDebug {
		fmt.Fprintln(text.NewIndentWriter(w, []byte("  ")), clean)
	} else {
		fmt.Fprintf(text.NewIndentWriter(w, []byte("  ")), "%v\n", trace)
	}
}
