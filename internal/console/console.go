// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package console

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kr/text"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/go-ids"
)

const (
	CatOutputTool = common.CatOutputTool
	CatOutputUs   = common.CatOutputUs
)

var (
	// Configured globally.
	DebugToConsole = false
)

func Stdout(ctx context.Context) io.Writer {
	return Output(ctx, common.KnownStdout)
}

func Stderr(ctx context.Context) io.Writer {
	return Output(ctx, common.KnownStderr)
}

func Output(ctx context.Context, name string) io.Writer {
	return TypedOutput(ctx, name, common.CatOutputTool)
}

func Debug(ctx context.Context) io.Writer {
	if DebugToConsole {
		return TypedOutput(ctx, "debug", common.CatOutputDebug)
	} else {
		return tasks.Attachments(ctx).Output(tasks.Output(string(common.CatOutputDebug), "text/plain"))
	}
}

func Warnings(ctx context.Context) io.Writer {
	return TypedOutput(ctx, "warning", common.CatOutputWarnings)
}

func Errors(ctx context.Context) io.Writer {
	return TypedOutput(ctx, "error", common.CatOutputErrors)
}

func TypedOutput(ctx context.Context, name string, cat common.CatOutputType) io.Writer {
	console := consoleOutputFromCtx(ctx, name, cat)
	stored := tasks.Attachments(ctx).Output(tasks.Output("console:"+name, "text/plain"))
	return io.MultiWriter(console, stored)
}

func consoleOutputFromCtx(ctx context.Context, name string, cat common.CatOutputType) io.Writer {
	unwrapped := UnwrapSink(tasks.SinkFrom(ctx))
	if t, ok := unwrapped.(writerLiner); ok {
		actionID := tasks.Attachments(ctx).ActionID()
		id := actionID.String()
		if actionID == "" {
			id = ids.NewRandomBase32ID(8)
		}

		if len(id) > 6 {
			id = id[:6]
		}

		buf := &consoleBuffer{actual: t, name: name, cat: cat, id: common.IdAndHashFrom(id)}
		if actionID != "" {
			buf.actionID = actionID
		}
		return buf
	}

	// If there's no console sink in context, pass along the original Stdout or Stderr.
	if name == common.KnownStdout {
		return os.Stdout
	} else if name == common.KnownStderr {
		return os.Stderr
	}

	return text.NewIndentWriter(os.Stdout, []byte(name+": "))
}

// ConsoleOutput returns a writer, whose output will be managed by the specified ConsoleSink.
func ConsoleOutput(console writerLiner, name string) io.Writer {
	return &consoleBuffer{actual: console, name: name}
}

func JSON(w io.Writer, message string, data interface{}) {
	fmt.Fprint(w, message, " ")
	enc := json.NewEncoder(w)
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(w, "<failed to serialize: %v>", err)
	}
}

func MakeConsoleName(logid string, key string, suffix string) string {
	if key != "" {
		if len(key) > 7 {
			key = key[:7]
		}
		key = key + " "
	}

	if len(logid) > 32 {
		logid = "..." + logid[len(logid)-29:]
	}

	return fmt.Sprintf("%s%s%s", key, logid, suffix)
}
