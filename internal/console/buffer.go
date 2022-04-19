// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package console

import (
	"bytes"
	"sync"

	"namespacelabs.dev/foundation/internal/console/common"
)

type writerLiner interface {
	WriteLines(common.IdAndHash, string, common.CatOutputType, [][]byte)
}

type consoleBuffer struct {
	actual writerLiner
	name   string
	cat    common.CatOutputType
	id     common.IdAndHash

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
			_, _ = w.buf.Read(data)
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
