package schedule

import (
	"errors"
	"regexp"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/document"
	"github.com/iilei/canned-schedules/schema"
)

const customPrefix = "x-"

// metaKey matches the text before the code span of a META item: "key:" with optional surrounding whitespace, the
// no-break space included (see document.IsSpace).
var metaKey = regexp.MustCompile(`^[\s\x{00A0}]*([^:\s\x{00A0}]+)[\s\x{00A0}]*:[\s\x{00A0}]*$`)

var metaSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	return schema.Compile(schema.MetaSchemaURL, schema.MetaSchema)
})

// meta is the decoded META block.
type meta struct {
	annotations   map[string]string
	schemaVersion string
	hint          string
}

// metaEntry is one decoded META item with the positions of its key and value.
type metaEntry struct {
	key      string
	value    string
	keyPos   diag.Pos
	valuePos diag.Pos
}

// decodeMeta reads the META block (if any) and validates it against the embedded META schema.
func decodeMeta(d *document.Directive) (meta, diag.List) {
	m := meta{schemaVersion: SchemaVersion, annotations: map[string]string{}}
	if d == nil {
		return m, nil
	}

	m.hint = d.Title.Value

	entries, diags := metaEntries(d)
	obj := make(map[string]any, len(entries))
	byKey := make(map[string]metaEntry, len(entries))

	for _, e := range entries {
		obj[e.key] = e.value
		byKey[e.key] = e
	}

	diags = append(diags, validateMeta(obj, byKey, d.KindPos)...)
	if diags.HasErrors() {
		return m, diags
	}

	for _, e := range entries {
		switch {
		case e.key == "schema-version":
			m.schemaVersion = e.value
		case strings.HasPrefix(e.key, customPrefix):
			m.annotations[e.key] = e.value
		}
	}

	return m, nil
}

func metaEntries(d *document.Directive) ([]metaEntry, diag.List) {
	var (
		entries []metaEntry
		diags   diag.List
		seen    = map[string]bool{}
	)

	for _, item := range d.Items {
		e, ok := metaItem(item)
		if !ok {
			diags = append(diags, diag.Errorf(item.Pos, diag.CodeMetaSchema,
				"a META item is a key, a colon and a value in backticks").WithExample("- x-owner: `team-xyz`"))

			continue
		}

		if seen[e.key] {
			diags = append(diags, diag.Errorf(e.keyPos, diag.CodeMetaSchema, "META key %q appears twice", e.key))

			continue
		}

		seen[e.key] = true
		entries = append(entries, e)
	}

	return entries, diags
}

// metaItem decodes `key: `value“ with nothing but whitespace after the code span.
func metaItem(item document.Item) (metaEntry, bool) {
	toks := item.Tokens
	if len(toks) == 3 && toks[2].Kind == document.TokenText && document.TrimSpace(toks[2].Value) == "" {
		toks = toks[:2]
	}

	if len(toks) != 2 || toks[0].Kind != document.TokenText || toks[1].Kind != document.TokenCode {
		return metaEntry{}, false
	}

	m := metaKey.FindStringSubmatchIndex(toks[0].Value)
	if m == nil {
		return metaEntry{}, false
	}

	keyPos := toks[0].Pos
	keyPos.Col += m[2]

	return metaEntry{
		key:      toks[0].Value[m[2]:m[3]],
		value:    toks[1].Value,
		keyPos:   keyPos,
		valuePos: toks[1].Pos,
	}, true
}

// validateMeta validates obj and points each schema error at the offending key or value.
func validateMeta(obj map[string]any, byKey map[string]metaEntry, blockPos diag.Pos) diag.List {
	sch, err := metaSchema()
	if err != nil {
		return diag.List{diag.Errorf(blockPos, diag.CodeMetaSchema, "META schema: %v", err)}
	}

	err = sch.Validate(obj)

	var ve *jsonschema.ValidationError
	if err == nil || !errors.As(err, &ve) {
		return nil
	}

	var diags diag.List

	for _, leaf := range leaves(ve) {
		diags = append(diags, metaDiag(leaf, byKey, blockPos)...)
	}

	diags.Sort()

	return diags
}

func metaDiag(leaf *jsonschema.ValidationError, byKey map[string]metaEntry, blockPos diag.Pos) diag.List {
	msg := leaf.Error()

	if len(leaf.InstanceLocation) > 0 {
		e := byKey[leaf.InstanceLocation[0]]

		return diag.List{diag.Errorf(e.valuePos, diag.CodeMetaSchema, "META %s: %s", e.key, msg)}
	}

	if ap, ok := leaf.ErrorKind.(*kind.AdditionalProperties); ok {
		var diags diag.List

		for _, key := range ap.Properties {
			diags = append(diags, diag.Errorf(byKey[key].keyPos, diag.CodeMetaSchema,
				"META key %q is not allowed; use schema-version or a key starting with %s", key, customPrefix).
				WithExample("- x-owner: `team-xyz`"))
		}

		return diags
	}

	return diag.List{diag.Errorf(blockPos, diag.CodeMetaSchema, "META: %s", msg)}
}

func leaves(ve *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(ve.Causes) == 0 {
		return []*jsonschema.ValidationError{ve}
	}

	var out []*jsonschema.ValidationError
	for _, c := range ve.Causes {
		out = append(out, leaves(c)...)
	}

	return out
}
