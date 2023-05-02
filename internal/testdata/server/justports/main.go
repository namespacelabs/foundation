package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello world")
	}))
	http.ListenAndServe(":3000", http.DefaultServeMux)
}
