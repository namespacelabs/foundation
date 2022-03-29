// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package console

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func TestBuffers(t *testing.T) {
	var liner bufferedLiner
	w := &consoleBuffer{actual: &liner, name: "foobar"}

	fmt.Fprint(w, "foo")
	fmt.Fprint(w, "bar")
	fmt.Fprintf(w, "baz\n")

	ev := liner.consume(t)

	assert.Equal(t, 1, len(ev.lines))
	assert.Equal(t, "foobarbaz", string(ev.lines[0]))

	fmt.Fprintln(w, "more lines")

	ev = liner.consume(t)

	assert.Equal(t, 1, len(ev.lines))
	assert.Equal(t, "more lines", string(ev.lines[0]))

	fmt.Fprintln(w, "one line\ntwo lines\nthree lines")

	ev = liner.consume(t)

	assert.Equal(t, 3, len(ev.lines))
	assert.Equal(t, "one line", string(ev.lines[0]))
	assert.Equal(t, "two lines", string(ev.lines[1]))
	assert.Equal(t, "three lines", string(ev.lines[2]))
}

type bufferedLiner struct {
	evs []bufferedEv
}

type bufferedEv struct {
	id    tasks.IdAndHash
	name  string
	lines [][]byte
}

func (w *bufferedLiner) WriteLines(id tasks.IdAndHash, name string, _ tasks.CatOutputType, lines [][]byte) {
	w.evs = append(w.evs, bufferedEv{id, name, lines})
}

func (w *bufferedLiner) consume(t *testing.T) bufferedEv {
	if len(w.evs) == 0 {
		t.Fatal("expected an event")
	}
	ev := w.evs[0]
	w.evs = w.evs[1:]
	return ev
}