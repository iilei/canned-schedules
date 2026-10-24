//go:build !(js && wasm)

// Command canned-schedules-wasm exposes the pipeline to the browser preflight page. It only does something useful
// when built for the browser:
//
//	GOOS=js GOARCH=wasm go build -o app.wasm ./cmd/canned-schedules-wasm
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "canned-schedules-wasm runs in the browser; build it with GOOS=js GOARCH=wasm")
	os.Exit(1)
}
