//go:build js && wasm

// Command canned-schedules-wasm exposes the pipeline to the browser preflight page as
// cannedSchedules.evaluate(markdown, args, linkArgs) → JSON string (see internal/web.Result).
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

	res := web.Evaluate([]byte(argv[0].String()), stringArgs(argv, 2), stringArgs(argv, 1))

	return res.Encode()
}

// sphinxKit(markdown, args, linkArgs) → Uint8Array with the zip, or an error message string.
func sphinxKit(_ js.Value, argv []js.Value) any {
	if len(argv) == 0 {
		return "missing document"
	}

	zip, err := web.SphinxKit([]byte(argv[0].String()), stringArgs(argv, 2), stringArgs(argv, 1))
	if err != nil {
		return err.Error()
	}

	arr := js.Global().Get("Uint8Array").New(len(zip))
	js.CopyBytesToJS(arr, zip)

	return arr
}

// stringArgs reads argv[n] as a list of strings; absent or undefined means none.
func stringArgs(argv []js.Value, n int) []string {
	var args []string

	if len(argv) > n && argv[n].Truthy() {
		for i := range argv[n].Length() {
			args = append(args, argv[n].Index(i).String())
		}
	}

	return args
}
