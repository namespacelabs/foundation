// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package simplelog

import (
	"bytes"
	"fmt"
	"io"

	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var AlsoReportStartEvents = false

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

	// Ignore errors
	_, _ = sl.out.Write(b.Bytes())
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

	// Ignore errors
	_, _ = sl.out.Write(b.Bytes())
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

	// Ignore errors
	_, _ = sl.out.Write(b.Bytes())
}

func (sl *logger) AttachmentsUpdated(tasks.ActionID, *tasks.ResultData) { /* nothing to do */ }

func (sl *logger) Output(name, contentType string, outputType common.CatOutputType) io.Writer {
	return nil
}
