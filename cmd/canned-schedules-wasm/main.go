//go:build js && wasm

// Command canned-schedules-wasm exposes the pipeline to the browser preflight page as
// cannedSchedules.evaluate(markdown, args) → JSON string (see internal/web.Result).
package main

import (
	"syscall/js"

	"github.com/iilei/canned-schedules/internal/web"
)

func main() {
	js.Global().Set("cannedSchedules", js.ValueOf(map[string]any{
		"evaluate": js.FuncOf(evaluate),
	}))

	select {} // Keep the Go runtime alive for later calls.
}

func evaluate(_ js.Value, argv []js.Value) any {
	if len(argv) == 0 {
		return ""
	}

	var args []string

	if len(argv) > 1 {
		for i := range argv[1].Length() {
			args = append(args, argv[1].Index(i).String())
		}
	}

	res := web.Evaluate([]byte(argv[0].String()), args)

	return res.Encode()
}
