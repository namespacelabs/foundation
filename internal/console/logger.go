// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package console

import (
	"io"
	"log"

	"gopkg.in/natefinch/lumberjack.v2"
)

type rotatedDebugLogger struct {
	rotatedFile *lumberjack.Logger
	logger      *log.Logger
}

func newRotatedDebugLogger(filePath string) io.WriteCloser {
	rotatedFile := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     10,   // days
		Compress:   true, // disabled by default
	}

	return &rotatedDebugLogger{
		rotatedFile: rotatedFile,
		logger:      log.New(rotatedFile, "", log.LstdFlags|log.Lmicroseconds),
	}
}

func (r *rotatedDebugLogger) Write(buf []byte) (int, error) {
	r.logger.Print(string(buf))
	return len(buf), nil
}

func (r *rotatedDebugLogger) Close() error {
	return r.rotatedFile.Close()
}
