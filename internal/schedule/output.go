package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/iilei/canned-schedules/schema"
)

var outputSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	return schema.Compile(schema.OutputSchemaURL, schema.OutputSchema)
})

// Finish validates the output against the embedded output schema. Call it again after adding warnings. A validation
// failure is a bug in this tool, not in the input.
func (o *Output) Finish() error {
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
