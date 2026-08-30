// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package panic

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
)

func PanicIfPanic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 1<<20)
				n := runtime.Stack(buf, true)
				fmt.Fprintf(os.Stderr, "panic while handling http request: %s %s\n%v\n\n%s", r.Method, r.URL.String(), err, buf[:n])
				os.Exit(7)
			}
		}()

		h.ServeHTTP(w, r)
	})
}
