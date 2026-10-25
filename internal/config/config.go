// Package config loads the built-in configuration, applies command-line overrides and validates the result.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/schema"
)

// Symbol presets (representation.ascii.symbols).
const (
	SymbolsASCII   = "ascii"
	SymbolsBraille = "braille"
	SymbolsStyles  = "styles"
)

// Symbol presets for representation.ascii.symbols. Both are single-width in every terminal: plain ASCII, and braille,
// whose East Asian Width is neutral (block elements like ▒▓█ are "ambiguous" and turn double-width in CJK
// terminals). The styles preset has no entry: it uses each style's own symbol.
var symbolPresets = map[string][]string{
	SymbolsASCII:   {".", ":", "=", "#"},
	SymbolsBraille: {"⠀", "⣀", "⣤", "⣶", "⣿"},
}

// Override replaces the value at Path with the JSON document in Value.
type Override struct {
	Path  string
	Value string
}

// Config is the validated configuration.
type Config struct {
	SchemaVersion  string         `json:"schema-version"`
	Representation Representation `json:"representation"`
	States         []State        `json:"states"`
}

// State is one capacity state. The order of states matters: index 0 is the default base.
type State struct {
	ID          string `json:"id"`
	DisplayName string `json:"display-name"`
}

// Representation holds everything visual.
type Representation struct {
	ASCII  ASCII  `json:"ascii"`
	Theme  Theme  `json:"theme"`
	Styles Styles `json:"styles"`
}

// ASCII configures the ASCII preview.
type ASCII struct {
	// Symbols picks the symbol preset: ascii (default), braille or styles.
	Symbols string `json:"symbols"`
}

// Theme holds page-level colours.
type Theme struct {
	Light Surface `json:"light"`
	Dark  Surface `json:"dark"`
}

// Surface is a background/label colour pair.
type Surface struct {
	Background string `json:"background"`
	Label      string `json:"label"`
}

// Styles are assigned to states round robin.
type Styles struct {
	States []Style `json:"states"`
}

// Style is the visual representation of a state.
type Style struct {
	Color  ColorPair `json:"color"`
	Symbol string    `json:"symbol"`
}

// ColorPair holds one colour per colour scheme.
type ColorPair struct {
	Light string `json:"light"`
	Dark  string `json:"dark"`
}

// Load returns the built-in configuration with the overrides applied in order.
// Any error diagnostic means the returned config is nil.
func Load(overrides []Override) (*Config, diag.List) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.Config))
	if err != nil {
		return nil, diag.List{diag.Errorf(diag.Pos{}, diag.CodeConfigSchema, "built-in config: %v", err)}
	}

	for _, o := range overrides {
		doc, err = apply(doc, o)
		if err != nil {
			return nil, diag.List{diag.Errorf(diag.Pos{}, diag.CodeInvalidOverride, "%v", err)}
		}
	}

	if err := validate(doc); err != nil {
		return nil, diag.List{diag.Errorf(diag.Pos{}, diag.CodeConfigSchema, "%v", err)}
	}

	cfg, err := decode(doc)
	if err != nil {
		return nil, diag.List{diag.Errorf(diag.Pos{}, diag.CodeConfigSchema, "%v", err)}
	}

	diags := cfg.check()
	if diags.HasErrors() {
		return nil, diags
	}

	return cfg, diags
}

// State returns the state whose id matches name case-insensitively.
func (c *Config) State(name string) (State, bool) {
	for _, t := range c.States {
		if strings.EqualFold(t.ID, name) {
			return t, true
		}
	}

	return State{}, false
}

// Symbol returns the ASCII preview symbol for the state at index i: from the preset in representation.ascii.symbols
// by state order (round robin), or the style's own symbol for the styles preset.
func (c *Config) Symbol(i int) string {
	preset := symbolPresets[c.Representation.ASCII.Symbols]
	if preset == nil {
		return c.Style(i).Symbol
	}

	return preset[i%len(preset)]
}

// Style returns the style assigned to the state at index i (round robin).
func (c *Config) Style(i int) Style {
	styles := c.Representation.Styles.States

	return styles[i%len(styles)]
}

// check runs the rules JSON Schema cannot express.
func (c *Config) check() diag.List {
	var diags diag.List

	seen := map[string]string{}
	for _, t := range c.States {
		key := strings.ToUpper(t.ID)
		if prev, ok := seen[key]; ok {
			diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeConfigDuplicateState,
				"state ids %q and %q collide (state ids are case-insensitive)", prev, t.ID))
		}
		seen[key] = t.ID
	}

	symbols := map[string]bool{}
	for _, s := range c.Representation.Styles.States {
		if c.Representation.ASCII.Symbols != SymbolsStyles {
			break // Style symbols are only used by the styles preset.
		}

		if symbols[s.Symbol] {
			diags = append(diags, diag.Warnf(diag.Pos{}, diag.CodeConfigDuplicateSymbol,
				"style symbol %q is used more than once: ASCII previews can tell those states apart only by colour",
				s.Symbol))
		}
		symbols[s.Symbol] = true
	}

	if n, m := len(c.Representation.Styles.States), len(c.States); n < m {
		diags = append(diags, diag.Warnf(diag.Pos{}, diag.CodeStyleReuse,
			"%d styles for %d states: styles repeat, so states become hard to tell apart", n, m))
	}

	return diags
}

func validate(doc any) error {
	sch, err := schema.Compile(schema.ConfigSchemaURL, schema.ConfigSchema)
	if err != nil {
		return err
	}

	return sch.Validate(doc)
}

func decode(doc any) (*Config, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	return &cfg, nil
}
