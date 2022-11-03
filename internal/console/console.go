// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package console

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kr/text"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/sync"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/tasks"
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
		return tasks.Attachments(ctx).Output(tasks.Output(string(common.CatOutputDebug), "text/plain"), common.CatOutputDebug)
	}
}

func Warnings(ctx context.Context) io.Writer {
	return TypedOutput(ctx, "warning", common.CatOutputWarnings)
}

func Errors(ctx context.Context) io.Writer {
	return TypedOutput(ctx, "error", common.CatOutputErrors)
}

func ConsoleOutputName(name string) tasks.OutputName {
	return tasks.Output("console:"+name, "application/json+ns-console-log")
}

func TypedOutput(ctx context.Context, name string, cat common.CatOutputType) io.Writer {
	stored := tasks.Attachments(ctx).Output(ConsoleOutputName(name), cat)
	return consoleOutputFromCtx(ctx, name, cat, writeStored{stored})
}

type writeStored struct {
	stored io.Writer
}

func (w writeStored) WriteLines(id common.IdAndHash, name string, cat common.CatOutputType, actionID tasks.ActionID, ts time.Time, lines [][]byte) {
	strLines := make([]string, len(lines))
	for k, line := range lines {
		strLines[k] = string(line)
	}

	if m, err := protojson.Marshal(&storage.LogLine{
		BufferId:       id.ID,
		BufferName:     name,
		ActionCategory: string(cat),
		ActionId:       actionID.String(),
		Line:           strLines,
		Timestamp:      timestamppb.New(ts),
	}); err == nil {
		_, _ = w.stored.Write(append(m, []byte("\n")...))
	} else {
		_, _ = w.stored.Write([]byte(`{"failure":"serialization failure"}\n`))
	}
}

func consoleOutputFromCtx(ctx context.Context, name string, cat common.CatOutputType, extra ...writesLines) io.Writer {
	unwrapped := UnwrapSink(tasks.SinkFrom(ctx))
	if t, ok := unwrapped.(writesLines); ok {
		actionID := tasks.Attachments(ctx).ActionID()
		id := actionID.String()
		if actionID == "" {
			id = ids.NewRandomBase32ID(8)
		}

		if len(id) > 6 {
			id = id[:6]
		}

		buf := &consoleBuffer{
			actual: append([]writesLines{t}, extra...),
			name:   name,
			cat:    cat,
			id:     common.IdAndHashFrom(id),
		}
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

	// IndentWriter is not thread safe.
	return sync.SyncWriter(text.NewIndentWriter(os.Stdout, []byte(name+": ")))
}

// ConsoleOutput returns a writer, whose output will be managed by the specified ConsoleSink.
func ConsoleOutput(console writesLines, name string) io.Writer {
	return &consoleBuffer{actual: []writesLines{console}, name: name}
}

func WriteJSON(w io.Writer, message string, data interface{}) {
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
