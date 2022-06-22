// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package consolesink

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/morikuni/aec"
	"namespacelabs.dev/foundation/internal/logoutput"
	"namespacelabs.dev/foundation/internal/text/timefmt"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Style struct {
	Header   aec.ANSI
	Category aec.ANSI
	Cached   aec.ANSI
	Progress aec.ANSI
	Argument aec.ANSI
	Result   aec.ANSI
	Notice   aec.ANSI
	Error    aec.ANSI
	Scope    aec.ANSI
}

var WithColors = Style{
	Header:   aec.LightBlackF,
	Category: aec.LightBlueF,
	Cached:   aec.LightBlackF,
	Progress: aec.LightBlackF,
	Argument: aec.CyanF,
	Result:   aec.BlueF,
	Notice:   aec.BlueF,
	Error:    aec.RedF,
	Scope:    aec.Italic,
}

var NoColors = Style{
	Header:   noOpANSI,
	Category: noOpANSI,
	Cached:   noOpANSI,
	Progress: noOpANSI,
	Argument: noOpANSI,
	Result:   noOpANSI,
	Notice:   noOpANSI,
	Error:    noOpANSI,
	Scope:    noOpANSI,
}

func (s Style) renderLine(w io.Writer, li lineItem) {
	data := li.data

	if data.State.IsDone() {
		// XXX using UTC() here to be consistent with zerolog.ConsoleWriter.
		t := data.Completed.UTC().Format(logoutput.StampMilliTZ)
		fmt.Fprint(w, s.Header.Apply(t), " ")

		if OutputActionID {
			fmt.Fprint(w, s.Header.Apply("["+data.ActionID.String()[:8]+"] "))
		}
	}

	if data.Category != "" {
		fmt.Fprint(w, s.Category.Apply("("+data.Category+") "))
	}

	name := data.HumanReadable
	if name == "" {
		name = data.Name
	}

	if li.cached {
		fmt.Fprint(w, s.Cached.Apply(name))
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
				fmt.Fprint(w, " ")
			}
			fmt.Fprint(&ws, pkg)
		}

		if origlen > 0 {
			fmt.Fprintf(&ws, " and %d more", origlen-len(scope))
		}

		fmt.Fprintf(w, " %s", s.Scope.Apply(ws.String()))
	}

	for _, kv := range li.serialized {
		color := s.Argument
		if kv.result {
			color = s.Result
		}
		fmt.Fprint(w, " ", color.Apply(kv.key+"="), kv.value)
	}

	if data.Err != nil {
		t := tasks.ErrorType(data.Err)
		if t == tasks.ErrTypeIsCancelled || t == tasks.ErrTypeIsDependencyFailed {
			fmt.Fprint(w, " ", s.Notice.Apply(string(t)))
		} else {
			fmt.Fprint(w, " ", s.Error.Apply("err="), s.Error.Apply(data.Err.Error()))
		}
	}
}

func (s Style) renderCompletedAction(raw io.Writer, r lineItem) {
	s.renderLine(raw, r)
	if !r.data.Started.IsZero() && !r.cached {
		if !r.data.Started.Equal(r.data.Created) {
			d := r.data.Started.Sub(r.data.Created)
			if d >= 1*time.Microsecond {
				fmt.Fprint(raw, " ", s.Header.Apply("waited="), timefmt.Format(d))
			}
		}

		d := r.data.Completed.Sub(r.data.Started)
		fmt.Fprint(raw, " ", s.Header.Apply("took="), timefmt.Format(d))
	}
	fmt.Fprintln(raw)
}

func (s Style) LogAction(w io.Writer, ev tasks.EventData) {
	item := lineItem{
		data: ev,
	}

	item.precompute()

	s.renderCompletedAction(w, item)
}

// An implementation of aec.ANSI that does completely nothing.
// It is more appropriate to use it in for non-TTY output since
// [aec.EmptyBuilder.ANSI] inserts reset codes "ESC[0m" regardless.
type noOpANSIImpl struct{}

func (noOpANSIImpl) String() string {
	return ""
}

func (noOpANSIImpl) With(as ...aec.ANSI) aec.ANSI {
	return aec.EmptyBuilder.ANSI.With(as...)
}

func (noOpANSIImpl) Apply(s string) string {
	return s
}

var noOpANSI = noOpANSIImpl{}
