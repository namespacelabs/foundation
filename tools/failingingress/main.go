package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

func main() {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Listening on %v", ln.Addr())

	s := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Failed")
	})}

	if err := s.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
