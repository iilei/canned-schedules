package diag

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestCodesComplete keeps the code table complete: every Code constant in diag.go is documented exactly once in
// Codes, with a sentence.
func TestCodesComplete(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "diag.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	declared := map[string]bool{}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}

		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Values) != 1 || !strings.HasPrefix(vs.Names[0].Name, "Code") {
				continue
			}

			if lit, ok := vs.Values[0].(*ast.BasicLit); ok {
				code, _ := strconv.Unquote(lit.Value)
				declared[code] = true
			}
		}
	}

	documented := map[string]int{}

	for _, c := range Codes {
		documented[c.Code]++

		if !declared[c.Code] {
			t.Errorf("Codes documents %q, which diag.go doesn't declare", c.Code)
		}

		if !strings.HasSuffix(c.Meaning, ".") || c.Severity == 0 {
			t.Errorf("%q: the meaning must be a sentence and the severity set", c.Code)
		}
	}

	for code := range declared {
		if documented[code] != 1 {
			t.Errorf("%q is documented %d times in Codes, want once", code, documented[code])
		}
	}
}
