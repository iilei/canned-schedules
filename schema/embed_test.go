package schema

import (
	"bytes"
	"encoding/json"
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

// TestVersionIsTheSingleSource keeps the JSON files in step with Version: every schema's $id is its URL constant,
// every schema-version constant and the built-in config carry Version, and the config points at its schema.
func TestVersionIsTheSingleSource(t *testing.T) {
	t.Parallel()

	for url, raw := range map[string][]byte{
		ConfigSchemaURL: ConfigSchema,
		MetaSchemaURL:   MetaSchema,
		OutputSchemaURL: OutputSchema,
		ReportSchemaURL: ReportSchema,
	} {
		var s struct {
			ID         string `json:"$id"`
			Properties struct {
				SchemaVersion struct {
					Const string `json:"const"`
				} `json:"schema-version"`
			} `json:"properties"`
		}

		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}

		if s.ID != url || s.Properties.SchemaVersion.Const != Version {
			t.Errorf("%s: $id %q, schema-version %q; want %q", url, s.ID, s.Properties.SchemaVersion.Const, Version)
		}
	}

	var cfg struct {
		Schema        string `json:"$schema"`
		SchemaVersion string `json:"schema-version"`
	}

	if err := json.Unmarshal(Config, &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Schema != ConfigSchemaURL || cfg.SchemaVersion != Version {
		t.Errorf("config.json: $schema %q, schema-version %q", cfg.Schema, cfg.SchemaVersion)
	}
}
