// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package consolesink

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/text/timefmt"
	"namespacelabs.dev/foundation/std/tasks"
)

const StampMilliTZ = "Jan _2 15:04:05.000 MST"

func renderTime(w io.Writer, s colors.Style, t time.Time) {
	// XXX using UTC() here to be consistent with zerolog.ConsoleWriter.
	str := t.UTC().Format(StampMilliTZ)
	fmt.Fprint(w, s.Header.Apply(str), " ")
}

func renderLine(w io.Writer, s colors.Style, li Renderable) {
	data := li.Data

	if OutputActionID {
		fmt.Fprint(w, s.Header.Apply("["+trim(data.ActionID.String(), 12)+"] "))
	}

	if data.Category != "" {
		fmt.Fprint(w, s.LogCategory.Apply("("+data.Category+") "))
	}

	name := data.HumanReadable
	if name == "" {
		name = data.Name
	}

	if li.Cached {
		fmt.Fprint(w, s.LogCachedName.Apply(name))
	} else {
		fmt.Fprint(w, name)
	}

	if progress := li.Progress; progress != nil && data.State == tasks.ActionRunning {
		if p := progress.FormatProgress(); p != "" {
			fmt.Fprint(w, " ", s.Progress.Apply(p))
		}
	}

	if data.HumanReadable == "" && len(li.Scope) > 0 {
		var ws bytes.Buffer

		scope := li.Scope
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

	for _, kv := range li.Serialized {
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

func renderCompletedAction(raw io.Writer, s colors.Style, r Renderable) {
	if r.Data.State.IsDone() {
		renderTime(raw, s, r.Data.Completed)
	} else {
		renderTime(raw, s, r.Data.Started)
	}

	renderLine(raw, s, r)
	if !r.Data.Started.IsZero() && !r.Cached {
		if !r.Data.Started.Equal(r.Data.Created) {
			d := r.Data.Started.Sub(r.Data.Created)
			if d >= 1*time.Microsecond {
				fmt.Fprint(raw, " ", s.Header.Apply("waited="), timefmt.Format(d))
			}
		}

		if r.Data.State.IsDone() {
			d := r.Data.Completed.Sub(r.Data.Started)
			fmt.Fprint(raw, " ", s.Header.Apply("took="), timefmt.Format(d))
		}
	}
	fmt.Fprintln(raw)
}

func LogAction(w io.Writer, s colors.Style, ev tasks.EventData) {
	item := Renderable{
		Data: ev,
	}

	item.precompute(tasks.ResultData{})

	renderCompletedAction(w, s, item)
}
