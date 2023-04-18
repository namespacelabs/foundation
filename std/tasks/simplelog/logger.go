// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package simplelog

import (
	"bytes"
	"fmt"
	"io"

	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/idtypes"
)

var AlsoReportStartEvents = false

func SetupFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&AlsoReportStartEvents, "also_report_start_events", AlsoReportStartEvents,
		"If set to true, we log a start event for each action, if --log_actions is also set.")

	_ = flags.MarkHidden("also_report_start_events")
}

func NewSink(w io.Writer, maxLevel int) tasks.ActionSink {
	return &logger{w, maxLevel}
}

type logger struct {
	out      io.Writer
	maxLevel int // Only display actions at this level or below (all actions are still computed).
}

func (sl logger) shouldLog(ev tasks.EventData) bool {
	// Don't emit logs for "compute.wait".
	if ev.AnchorID != "" {
		return false
	}
	return ev.Level <= sl.maxLevel
}

func (sl *logger) Waiting(ra *tasks.RunningAction) {
	// Do nothing.
}

func (sl logger) write(b []byte) {
	// Ignore errors
	_, _ = sl.out.Write(b)
}

func (sl *logger) Started(ra *tasks.RunningAction) {
	if !tasks.LogActions || !AlsoReportStartEvents {
		return
	}

	if !sl.shouldLog(ra.Data) {
		return
	}

	var b bytes.Buffer
	fmt.Fprint(&b, "↦ ")
	consolesink.LogAction(&b, colors.NoColors, ra.Data)

	sl.write(b.Bytes())
}

func (sl *logger) Done(ra *tasks.RunningAction) {
	if !tasks.LogActions {
		return
	}
	if !sl.shouldLog(ra.Data) {
		return
	}

	var b bytes.Buffer
	if AlsoReportStartEvents {
		if ra.Data.Err == nil {
			fmt.Fprint(&b, "✔ ")
		} else {
			fmt.Fprint(&b, "✘ ")
		}
	}
	consolesink.LogAction(&b, colors.NoColors, ra.Data)

	sl.write(b.Bytes())
}

func (sl *logger) Instant(ev *tasks.EventData) {
	if !tasks.LogActions {
		return
	}
	if !sl.shouldLog(*ev) {
		return
	}

	var b bytes.Buffer
	if AlsoReportStartEvents {
		// We use checkboxes here to distinguish Instant() vs Done()
		if ev.Err == nil {
			fmt.Fprint(&b, "☑ ")
		} else {
			fmt.Fprint(&b, "☒ ")
		}
	}
	consolesink.LogAction(&b, colors.NoColors, *ev)

	sl.write(b.Bytes())
}

func (sl *logger) AttachmentsUpdated(tasks.ActionID, *tasks.ResultData) { /* nothing to do */ }

func (sl *logger) Output(name, contentType string, outputType idtypes.CatOutputType) io.Writer {
	return nil
}
