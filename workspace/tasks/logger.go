// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"fmt"

	"github.com/rs/zerolog"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/text/timefmt"
)

var AlsoReportStartEvents = false

func NewJsonLoggerSink(logger zerolog.Logger, maxLevel int) ActionSink {
	return &jsonLogger{logger, maxLevel}
}

type jsonLogger struct {
	logger   zerolog.Logger
	maxLevel int // Only display actions at this level or below (all actions are still computed).
}

func (sl *jsonLogger) event(ev EventData, withArgs bool) *zerolog.Event {
	e := sl.logger.Info().Str("action_id", ev.ActionID.String()).Int("log_level", ev.Level)
	if ev.ParentID != "" {
		e = e.Str("parent_id", ev.ParentID.String())
	}
	if withArgs {
		if ev.Scope.Len() > 0 {
			e = e.Strs("scope", ev.Scope.PackageNamesAsString())
		}

		for _, arg := range ev.Arguments {
			res, err := common.Serialize(arg.Msg)
			if err != nil {
				e = e.Interface(arg.Name, fmt.Sprintf("failed to serialize: %v", err))
			} else {
				e = e.Interface(arg.Name, res)
			}
		}
	}
	if AlsoReportStartEvents {
		e = e.Str("name", ev.Name)
	}
	return e
}

func (sl jsonLogger) shouldLog(ev EventData) bool {
	return ev.Level <= sl.maxLevel
}

func (sl *jsonLogger) Waiting(ra *RunningAction) {
	// Do nothing.
}

func (sl *jsonLogger) Started(ra *RunningAction) {
	if !AlsoReportStartEvents {
		return
	}

	if !sl.shouldLog(ra.Data) {
		return
	}

	sl.event(ra.Data, true).Msg("start")
}

func (sl *jsonLogger) Done(ra *RunningAction) {
	if !sl.shouldLog(ra.Data) {
		return
	}

	ev := sl.event(ra.Data, true)
	if ra.Data.Err != nil {
		t := ErrorType(ra.Data.Err)
		switch t {
		case ErrTypeIsCancelled, ErrTypeIsDependencyFailed:
			ev.Str("reason", string(t))
			return
		default:
			ev = ev.Stack().Err(ra.Data.Err)
		}
	}

	ev = ev.Str("took", timefmt.Format(ra.Data.Completed.Sub(ra.Data.Started)))

	if AlsoReportStartEvents {
		ev.Msg("done")
	} else {
		ev.Msg(ra.Data.Name)
	}
}

func (sl *jsonLogger) Instant(ev *EventData) {
	if !sl.shouldLog(*ev) {
		return
	}

	sl.event(*ev, true).Msg(ev.Name)
}

func (sl *jsonLogger) AttachmentsUpdated(ActionID, *ResultData) { /* nothing to do */ }
