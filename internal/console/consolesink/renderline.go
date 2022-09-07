// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package consolesink

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/text/timefmt"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const StampMilliTZ = "Jan _2 15:04:05.000 MST"

func renderTime(w io.Writer, s colors.Style, t time.Time) {
	// XXX using UTC() here to be consistent with zerolog.ConsoleWriter.
	str := t.UTC().Format(StampMilliTZ)
	fmt.Fprint(w, s.Header.Apply(str), " ")
}

func renderLine(w io.Writer, s colors.Style, li lineItem) {
	data := li.data

	if data.State.IsDone() {
		renderTime(w, s, data.Completed)

		if OutputActionID {
			fmt.Fprint(w, s.Header.Apply("["+data.ActionID.String()[:8]+"] "))
		}

		fmt.Fprint(w, "✓ ")
	} else {
		renderTime(w, s, data.Started)

		if OutputActionID {
			fmt.Fprint(w, s.Header.Apply("["+data.ActionID.String()[:8]+"] "))
		}

		fmt.Fprint(w, "↦ ")
	}

	if data.Category != "" {
		fmt.Fprint(w, s.LogCategory.Apply("("+data.Category+") "))
	}

	name := data.HumanReadable
	if name == "" {
		name = data.Name
	}

	if li.cached {
		fmt.Fprint(w, s.LogCachedName.Apply(name))
	} else {
		fmt.Fprint(w, name)
	}

	if progress := li.progress; progress != nil && data.State == tasks.ActionRunning {
		if p := progress.FormatProgress(); p != "" {
			fmt.Fprint(w, " ", s.Progress.Apply(p))
		}
	}

	if data.HumanReadable == "" && len(li.scope) > 0 {
		var ws bytes.Buffer

		scope := li.scope
		var origlen int
		if len(scope) > 3 {
			origlen = len(scope)
			scope = scope[:3]
		}

		for k, pkg := range scope {
			if k > 0 {
				fmt.Fprint(&ws, " ")
			}
			fmt.Fprint(&ws, pkg)
		}

		if origlen > 0 {
			fmt.Fprintf(&ws, " and %d more", origlen-len(scope))
		}

		fmt.Fprintf(w, " %s", s.LogScope.Apply(ws.String()))
	}

	for _, kv := range li.serialized {
		color := s.LogArgument
		if kv.result {
			color = s.LogResult
		}
		fmt.Fprint(w, " ", color.Apply(kv.key+"="), kv.value)
	}

	if data.Err != nil {
		t := tasks.ErrorType(data.Err)
		if t == tasks.ErrTypeIsCancelled || t == tasks.ErrTypeIsDependencyFailed {
			fmt.Fprint(w, " ", s.LogErrorReason.Apply(string(t)))
		} else {
			fmt.Fprint(w, " ", s.LogError.Apply("err="), s.LogError.Apply(data.Err.Error()))
		}
	}
}

func renderCompletedAction(raw io.Writer, s colors.Style, r lineItem) {
	renderLine(raw, s, r)
	if !r.data.Started.IsZero() && !r.cached {
		if !r.data.Started.Equal(r.data.Created) {
			d := r.data.Started.Sub(r.data.Created)
			if d >= 1*time.Microsecond {
				fmt.Fprint(raw, " ", s.Header.Apply("waited="), timefmt.Format(d))
			}
		}

		if r.data.State.IsDone() {
			d := r.data.Completed.Sub(r.data.Started)
			fmt.Fprint(raw, " ", s.Header.Apply("took="), timefmt.Format(d))
		}
	}
	fmt.Fprintln(raw)
}

func LogAction(w io.Writer, s colors.Style, ev tasks.EventData) {
	item := lineItem{
		data: ev,
	}

	item.precompute()

	renderCompletedAction(w, s, item)
}
