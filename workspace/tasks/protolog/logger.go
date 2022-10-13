// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protolog

import (
	"io"
	"time"

	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewSink(ch chan *Log) *logger {
	return &logger{ch: ch}
}

var _ tasks.ActionSink = &logger{}

type logger struct {
	ch chan *Log
}

func (l *logger) Close() {
	close(l.ch)
}

func (l *logger) Waiting(ra *tasks.RunningAction) {
	l.ch <- &Log{
		LogLevel: int32(ra.Data.Level),
		Task:     ra.Proto(),
		Purpose:  Log_PURPOSE_WAITING,
	}
}

func (l *logger) Started(ra *tasks.RunningAction) {
	l.ch <- &Log{
		LogLevel: int32(ra.Data.Level),
		Task:     ra.Proto(),
		Purpose:  Log_PURPOSE_STARTED,
	}
}

func (l *logger) Done(ra *tasks.RunningAction) {
	l.ch <- &Log{
		LogLevel: int32(ra.Data.Level),
		Task:     ra.Proto(),
		Purpose:  Log_PURPOSE_DONE,
	}
}

func (l *logger) Instant(ev *tasks.EventData) {
	l.ch <- &Log{
		LogLevel: int32(ev.Level),
		Task:     ev.Proto(),
		Purpose:  Log_PURPOSE_INSTANT,
	}
}

func (l *logger) AttachmentsUpdated(tasks.ActionID, *tasks.ResultData) { /* nothing to do */ }

func (l *logger) Output(name, contentType string, outputType common.CatOutputType) io.Writer {
	return nil
}

func (l *logger) WriteLines(id common.IdAndHash, name string, cat common.CatOutputType, actionID tasks.ActionID, ts time.Time, lines [][]byte) {
	l.ch <- &Log{
		Lines: &Log_Lines{
			Name:  name,
			Cat:   string(cat),
			Lines: lines,
		},
	}
}
