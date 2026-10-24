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
		"evaluate":  js.FuncOf(evaluate),
		"sphinxKit": js.FuncOf(sphinxKit),
	}))

	select {} // Keep the Go runtime alive for later calls.
}

func evaluate(_ js.Value, argv []js.Value) any {
	if len(argv) == 0 {
		return ""
	}

	res := web.Evaluate([]byte(argv[0].String()), stringArgs(argv))

	return res.Encode()
}

// sphinxKit(markdown, args) → Uint8Array with the zip, or an error message string.
func sphinxKit(_ js.Value, argv []js.Value) any {
	if len(argv) == 0 {
		return "missing document"
	}

	zip, err := web.SphinxKit([]byte(argv[0].String()), stringArgs(argv))
	if err != nil {
		return err.Error()
	}

	arr := js.Global().Get("Uint8Array").New(len(zip))
	js.CopyBytesToJS(arr, zip)

	return arr
}

func stringArgs(argv []js.Value) []string {
	var args []string

	if len(argv) > 1 {
		for i := range argv[1].Length() {
			args = append(args, argv[1].Index(i).String())
		}
	}

	return args
}
