// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package simplelog

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

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
	return &logger{out: w, maxLevel: maxLevel}
}

type logger struct {
	mu       sync.Mutex
	out      io.Writer
	maxLevel int // Only display actions at this level or below (all actions are still computed).
}

func (sl *logger) shouldLog(ev tasks.EventData) bool {
	// Don't emit logs for "compute.wait".
	if ev.AnchorID != "" {
		return false
	}
	return ev.Level <= sl.maxLevel
}

func (sl *logger) Waiting(ra *tasks.RunningAction) {
	// Do nothing.
}

func (sl *logger) write(b []byte) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
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
	consolesink.LogAction(&b, colors.NoColors, consolesink.OutputActionID, ra.Data)

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
	consolesink.LogAction(&b, colors.NoColors, consolesink.OutputActionID, ra.Data)

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
	consolesink.LogAction(&b, colors.NoColors, consolesink.OutputActionID, *ev)

	sl.write(b.Bytes())
}

func (sl *logger) AttachmentsUpdated(tasks.ActionID, *tasks.ResultData) { /* nothing to do */ }

func (sl *logger) Output(name, contentType string, outputType idtypes.CatOutputType) io.Writer {
	return nil
}

func (sl *logger) WriteLines(_ idtypes.IdAndHash, name string, cat idtypes.CatOutputType, _ tasks.ActionID, _ time.Time, lines [][]byte) {
	var buf bytes.Buffer
	for _, line := range lines {
		switch name {
		case idtypes.KnownStdout, idtypes.KnownStderr:
			fmt.Fprintf(&buf, "%s\n", line)
		default:
			fmt.Fprintf(&buf, "%s: %s\n", name, line)
		}
	}

	// preserve default from consoleOutputFromCtx
	var w io.Writer = os.Stdout

	switch name {
	case idtypes.KnownStdout:
		w = os.Stdout

	case idtypes.KnownStderr:
		w = sl.out
	}

	switch cat {
	case idtypes.CatOutputDebug, idtypes.CatOutputInfo, idtypes.CatOutputWarnings, idtypes.CatOutputErrors:
		w = sl.out
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()
	w.Write(buf.Bytes())
}
