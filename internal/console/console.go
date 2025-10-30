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
	"path/filepath"
	"time"

	"github.com/kr/text"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/natefinch/lumberjack.v2"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/sync"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/idtypes"
	"namespacelabs.dev/go-ids"
)

const (
	CatOutputTool = idtypes.CatOutputTool
	CatOutputUs   = idtypes.CatOutputUs
)

var (
	// Configured globally.
	DebugToConsole = false
	DebugToFile    string
	RotatedFile    = false

	debugToWriter io.WriteCloser
)

func Prepare() error {
	if DebugToFile != "" && DebugToConsole {
		return fnerrors.Newf("--debug_to_console and --debug_to_file are exclusive")
	}

	if DebugToFile != "" {
		if err := os.MkdirAll(filepath.Dir(DebugToFile), 0755); err != nil {
			return err
		}

		rotatedFile := &lumberjack.Logger{
			Filename:   DebugToFile,
			MaxSize:    10, // megabytes
			MaxBackups: 3,
			MaxAge:     10,   // days
			Compress:   true, // disabled by default
		}

		debugToWriter = rotatedFile
	}

	return nil
}

func Cleanup() {
	if debugToWriter != nil {
		_ = debugToWriter.Close()
	}
}

func Stdout(ctx context.Context) io.Writer {
	return Output(ctx, idtypes.KnownStdout)
}

func Stderr(ctx context.Context) io.Writer {
	return Output(ctx, idtypes.KnownStderr)
}

func Output(ctx context.Context, name string) io.Writer {
	return TypedOutput(ctx, name, idtypes.CatOutputTool)
}

func Debug(ctx context.Context) io.Writer {
	return NamedDebug(ctx, "debug")
}

func NamedDebug(ctx context.Context, name string) io.Writer {
	return typedOutput(ctx, true, name, idtypes.CatOutputDebug)
}

func Info(ctx context.Context) io.Writer {
	return typedOutput(ctx, true, "info", idtypes.CatOutputInfo)
}

func Warnings(ctx context.Context) io.Writer {
	return typedOutput(ctx, true, "warning", idtypes.CatOutputWarnings)
}

func Errors(ctx context.Context) io.Writer {
	return typedOutput(ctx, true, "error", idtypes.CatOutputErrors)
}

func ConsoleOutputName(name string) tasks.OutputName {
	return tasks.Output("console:"+name, "application/json+ns-console-log")
}

func TypedOutput(ctx context.Context, name string, cat idtypes.CatOutputType) io.Writer {
	return typedOutput(ctx, false, name, cat)
}

func typedOutput(ctx context.Context, stderr bool, name string, cat idtypes.CatOutputType) io.Writer {
	if debugToWriter != nil {
		return debugToWriter
	}

	if cat == idtypes.CatOutputDebug && !DebugToConsole {
		return tasks.Attachments(ctx).Output(tasks.Output(name, "text/plain"), cat)
	}

	stored := tasks.Attachments(ctx).Output(ConsoleOutputName(name), cat)
	return consoleOutputFromCtx(ctx, stderr, name, cat, writeStored{stored})
}

type writeStored struct {
	stored io.Writer
}

func (w writeStored) WriteLines(id idtypes.IdAndHash, name string, cat idtypes.CatOutputType, actionID tasks.ActionID, ts time.Time, lines [][]byte) {
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

func consoleOutputFromCtx(ctx context.Context, stderr bool, name string, cat idtypes.CatOutputType, extra ...writesLines) io.Writer {
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
			id:     idtypes.IdAndHashFrom(id),
		}
		if actionID != "" {
			buf.actionID = actionID
		}
		return buf
	}

	// If there's no console sink in context, pass along the original Stdout or Stderr.
	switch name {
	case idtypes.KnownStdout:
		return os.Stdout

	case idtypes.KnownStderr:
		return os.Stderr
	}

	out := os.Stdout
	if stderr {
		out = os.Stderr
	}

	// IndentWriter is not thread safe.
	return sync.SyncWriter(text.NewIndentWriter(out, []byte(name+": ")))
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

func DebugWithTimestamp(ctx context.Context, format string, args ...any) {
	logLine := fmt.Sprintf(format, args...)
	fmt.Fprintf(Debug(ctx), "%s %s", time.Now().Format(time.RFC3339Nano), logLine)
}
