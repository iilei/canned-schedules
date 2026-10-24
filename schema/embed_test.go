package schema

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestGoldenOutputsMatchSchema keeps the fixtures (the spec) and the output schema from drifting apart.
func TestGoldenOutputsMatchSchema(t *testing.T) {
	t.Parallel()

	sch, err := Compile(OutputSchemaURL, OutputSchema)
	if err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob("../testdata/golden/*/*/want.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}

		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}

		if err := sch.Validate(doc); err != nil {
			t.Errorf("%s does not match the output schema: %v", f, err)
		}
	}
}

func TestEmbeddedSchemasCompile(t *testing.T) {
	t.Parallel()

	for url, raw := range map[string][]byte{
		ConfigSchemaURL: ConfigSchema,
		MetaSchemaURL:   MetaSchema,
		OutputSchemaURL: OutputSchema,
		ReportSchemaURL: ReportSchema,
	} {
		if _, err := Compile(url, raw); err != nil {
			t.Errorf("%s: %v", url, err)
		}
	}
}
