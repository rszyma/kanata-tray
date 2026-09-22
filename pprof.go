//go:build pprof

// This allows check for stuck goroutines like so:
// 1. nix develop --command go run -tags=pprof . --log-level 1
// 2. curl http://localhost:6060/debug/pprof/goroutine?debug=1

package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
)

func init() {
	fmt.Print("Profiling is enabled!\n")
	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
