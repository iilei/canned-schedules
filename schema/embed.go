// Package schema embeds the built-in configuration and the JSON Schemas that validate configs and document META.
package schema

import (
	"bytes"
	_ "embed" // Required for go:embed.
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ConfigSchemaURL is the $id of the config schema.
const ConfigSchemaURL = "https://raw.githubusercontent.com/iilei/canned-schedules/master/schema/config.schema.json"

// MetaSchemaURL is the $id of the document META schema.
const MetaSchemaURL = "https://raw.githubusercontent.com/iilei/canned-schedules/master/schema/meta.schema.json"

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
