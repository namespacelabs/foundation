// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package console

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"sync"

	"github.com/kr/text"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Stdout(ctx context.Context) io.Writer {
	return Output(ctx, tasks.KnownStdout)
}

func Stderr(ctx context.Context) io.Writer {
	return Output(ctx, tasks.KnownStderr)
}

func Output(ctx context.Context, name string) io.Writer {
	return TypedOutput(ctx, name, tasks.CatOutputTool)
}

func Warnings(ctx context.Context) io.Writer {
	return TypedOutput(ctx, "warnings", tasks.CatOutputWarnings)
}

func Errors(ctx context.Context) io.Writer {
	return TypedOutput(ctx, "errors", tasks.CatOutputErrors)
}

func TypedOutput(ctx context.Context, name string, cat tasks.CatOutputType) io.Writer {
	console := tasks.ConsoleOf(tasks.SinkFrom(ctx))
	if console == nil {
		if name == tasks.KnownStdout {
			return os.Stdout
		} else if name == tasks.KnownStderr {
			return os.Stderr
		}
		return text.NewIndentWriter(os.Stdout, []byte(name+": "))
	}

	id := tasks.Attachments(ctx).ActionID()
	if id == "" {
		id = smallRandID()
	}

	if len(id) > 6 {
		id = id[:6]
	}

	return &consoleBuffer{actual: console, name: name, cat: cat, id: tasks.IdAndHashFrom(id)}
}

func smallRandID() string {
	data := make([]byte, 8)
	rand.Read(data)
	return base64.RawStdEncoding.EncodeToString(data)
}

func ConsoleOutputWith(console *tasks.ConsoleSink, name string) io.Writer {
	return &consoleBuffer{actual: console, name: name}
}

type writerLiner interface {
	WriteLines(tasks.IdAndHash, string, tasks.CatOutputType, [][]byte)
}

type consoleBuffer struct {
	actual writerLiner
	name   string
	cat    tasks.CatOutputType
	id     tasks.IdAndHash

	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *consoleBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.buf.Write(p)
	var lines [][]byte
	for {
		if i := bytes.IndexByte(w.buf.Bytes(), '\n'); i >= 0 {
			data := make([]byte, i+1)
			w.buf.Read(data)
			line := dropCR(data[0 : len(data)-1]) // Drop the \n and the \r.
			lines = append(lines, line)
		} else {
			break
		}
	}
	w.mu.Unlock()
	if len(lines) > 0 {
		w.actual.WriteLines(w.id, w.name, w.cat, lines)
	}
	return len(p), nil
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}