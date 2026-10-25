package schedule

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/iilei/canned-schedules/schema"
)

const digestPrefix = "sha256:"

var outputSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	return schema.Compile(schema.OutputSchemaURL, schema.OutputSchema)
})

// Finish computes the digests and validates the output against the embedded output schema. Call it again after
// adding warnings. A validation failure is a bug in this tool, not in the input.
func (o *Output) Finish() error {
	schedule, err := json.Marshal(struct {
		Base string `json:"base"`
		Week Week   `json:"week"`
	}{Base: o.Base, Week: o.Week})
	if err != nil {
		return fmt.Errorf("encode schedule digest input: %w", err)
	}

	o.Digests = Digests{Schedule: digest(schedule)}

	raw, err := json.Marshal(o)
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("decode output: %w", err)
	}

	sch, err := outputSchema()
	if err != nil {
		return err
	}

	return sch.Validate(doc)
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)

	return digestPrefix + hex.EncodeToString(sum[:])
}
