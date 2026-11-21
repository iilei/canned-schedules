//go:build js && wasm

// Command canned-schedules-wasm exposes the pipeline to the browser preflight page as
// cannedSchedules.evaluate(markdown, args, linkArgs, siteArgs) → JSON string (see internal/web.Result), and checks a
// self-hosted playground's setup as cannedSchedules.checkSite(siteJSON) → JSON string (internal/web.SiteResult).
package main

import (
	"syscall/js"

	"github.com/iilei/canned-schedules/internal/web"
)

func main() {
	js.Global().Set("cannedSchedules", js.ValueOf(map[string]any{
		"evaluate":  js.FuncOf(evaluate),
		"sphinxKit": js.FuncOf(sphinxKit),
		"checkSite": js.FuncOf(checkSite),
	}))

	select {} // Keep the Go runtime alive for later calls.
}

func evaluate(_ js.Value, argv []js.Value) any {
	if len(argv) == 0 {
		return ""
	}

	res := web.EvaluateSite([]byte(argv[0].String()), stringArgs(argv, 3), stringArgs(argv, 2), stringArgs(argv, 1))

	return res.Encode()
}

// checkSite(siteJSON) → JSON string with the checked site or the errors.
func checkSite(_ js.Value, argv []js.Value) any {
	raw := ""
	if len(argv) > 0 {
		raw = argv[0].String()
	}

	res := web.CheckSite([]byte(raw))

	return res.Encode()
}

// sphinxKit(markdown, args, linkArgs, siteArgs) → Uint8Array with the zip, or an error message string.
func sphinxKit(_ js.Value, argv []js.Value) any {
	if len(argv) == 0 {
		return "missing document"
	}

	zip, err := web.SphinxKitSite([]byte(argv[0].String()), stringArgs(argv, 3), stringArgs(argv, 2),
		stringArgs(argv, 1))
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
