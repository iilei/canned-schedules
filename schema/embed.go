// Package schema embeds the built-in configuration and the JSON Schemas that validate configs and document META.
package schema

import _ "embed" // Required for go:embed.

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
