// Package schema embeds the built-in configuration and the JSON Schemas that validate configs and document META.
package schema

import (
	"bytes"
	_ "embed" // Required for go:embed.
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Version is the schema version: the date of the last breaking change to any of the contracts (document META,
// config, output, error report). Compatible additions keep the date; see "Stability" in the README.
const Version = "2026-10-25"

// BaseURL is where the schemas of this version are published (GitHub Pages), and the prefix of their $ids. The tool
// never fetches it: the schemas are embedded.
const BaseURL = "https://iilei.github.io/canned-schedules/schema/" + Version + "/"

// The $ids of the embedded schemas.
const (
	ConfigSchemaURL = BaseURL + "config.schema.json"
	OutputSchemaURL = BaseURL + "output.schema.json"
	ReportSchemaURL = BaseURL + "report.schema.json"
	MetaSchemaURL   = BaseURL + "meta.schema.json"
)

var (
	// Config is the built-in configuration.
	//
	//go:embed config.json
	Config []byte

	// ConfigSchema validates Config and any overrides applied to it.
	//
	//go:embed config.schema.json
	ConfigSchema []byte

	// MetaSchema validates the META block of a schedule document.
	//
	//go:embed meta.schema.json
	MetaSchema []byte

	// OutputSchema describes the canonical output; the tool validates its own output against it.
	//
	//go:embed output.schema.json
	OutputSchema []byte

	// ReportSchema describes the JSON error report.
	//
	//go:embed report.schema.json
	ReportSchema []byte
)

// Compile compiles the embedded schema raw under its $id url. Nothing is fetched over the network.
func Compile(url string, raw []byte) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", url, err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("schema %s: %w", url, err)
	}

	sch, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", url, err)
	}

	return sch, nil
}
