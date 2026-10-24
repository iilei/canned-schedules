package main

import (
	"bytes"
	"strings"
	"testing"
)

const basic = "../../testdata/golden/valid/basic/input.md"

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantStderr string
		args       []string
		wantCode   int
	}{
		{name: "valid file", args: []string{basic}, wantCode: exitOK},
		{name: "flags then file", args: []string{"--granularity", "60", basic}, wantCode: exitOK},
		{name: "no file", args: nil, wantCode: exitUsage, wantStderr: "missing input file"},
		{name: "stdin dash is not a file", args: []string{"-"}, wantCode: exitUsage, wantStderr: "no such file"},
		{name: "two files", args: []string{basic, basic}, wantCode: exitUsage, wantStderr: "expected one input file"},
		{
			name:       "flag after file",
			args:       []string{basic, "--granularity", "60"},
			wantCode:   exitUsage,
			wantStderr: "flags must come before the file",
		},
		{name: "missing file", args: []string{"nope.md"}, wantCode: exitUsage, wantStderr: "nope.md"},
		{
			name:       "document error reports file:line:col",
			args:       []string{"../../testdata/golden/error/time-24-00/input.md"},
			wantCode:   exitInvalid,
			wantStderr: "../../testdata/golden/error/time-24-00/input.md:9:17: error[invalid-time]",
		},
		{name: "invalid flag value", args: []string{"--granularity", "7", basic}, wantCode: exitInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}

			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}

			if (code == exitOK) != (stdout.Len() > 0) {
				t.Errorf("stdout must hold the JSON exactly when the run succeeds, got %d bytes", stdout.Len())
			}
		})
	}
}

