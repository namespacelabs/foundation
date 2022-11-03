// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package sync

import (
	"io"
	"sync"
)

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func SyncWriter(w io.Writer) io.Writer {
	return &syncWriter{w: w}
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
