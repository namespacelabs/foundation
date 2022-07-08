// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"log"
	"os"
)

// An example of a logger that implements `pkg/log/Logger`. Logs to
// stdout. If Debug == false then won't output anything.
type Logger struct {
	Debug   bool
	logsink *log.Logger
}

func NewLogger(debug bool) *Logger {
	return &Logger{
		Debug:   debug,
		logsink: log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds),
	}

}

func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.Debug {
		l.logsink.Printf(format, args...)
	}
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.logsink.Printf(format, args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logsink.Printf(format, args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logsink.Printf(format, args...)
}
