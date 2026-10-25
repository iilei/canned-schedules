package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var errEmptyPath = errors.New("empty path")

// segment is one step of an override path: an object key or an array index.
type segment struct {
	key   string
	index int
	isKey bool
}

// apply replaces the value at o.Path. The path must already exist: overrides never create keys or array elements.
func apply(doc any, o Override) (any, error) {
	path, err := parsePath(o.Path)
	if err != nil {
		return nil, fmt.Errorf("--set-config-at %q: %w", o.Path, err)
	}

	value, err := jsonschema.UnmarshalJSON(strings.NewReader(o.Value))
	if err != nil {
		return nil, fmt.Errorf("--set-config-value for %q is not valid JSON: %w", o.Path, err)
	}

	return replace(doc, path, value, o.Path)
}

func replace(node any, path []segment, value any, full string) (any, error) {
	if len(path) == 0 {
		return value, nil
	}

	seg := path[0]

	if seg.isKey {
		obj, ok := node.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("--set-config-at %q: %q is not an object key here", full, seg.key)
		}

		child, ok := obj[seg.key]
		if !ok {
			return nil, fmt.Errorf("--set-config-at %q: unknown key %q", full, seg.key)
		}

		newChild, err := replace(child, path[1:], value, full)
		if err != nil {
			return nil, err
		}

		obj[seg.key] = newChild

		return obj, nil
	}

	arr, ok := node.([]any)
	if !ok {
		return nil, fmt.Errorf("--set-config-at %q: [%d] used on a non-array value", full, seg.index)
	}

	if seg.index >= len(arr) {
		return nil, fmt.Errorf("--set-config-at %q: index %d out of range (length %d)", full, seg.index, len(arr))
	}

	newChild, err := replace(arr[seg.index], path[1:], value, full)
	if err != nil {
		return nil, err
	}

	arr[seg.index] = newChild

	return arr, nil
}

// parsePath parses dot-separated keys with optional [index] suffixes, e.g. "representation.styles.states[0]".
func parsePath(p string) ([]segment, error) {
	if p == "" {
		return nil, errEmptyPath
	}

	var path []segment

	for part := range strings.SplitSeq(p, ".") {
		key, rest, _ := strings.Cut(part, "[")
		if !validKey(key) {
			return nil, fmt.Errorf("invalid key %q", key)
		}

		path = append(path, segment{key: key, isKey: true})

		if rest == "" {
			continue
		}

		indexes, err := parseIndexes("[" + rest)
		if err != nil {
			return nil, err
		}

		path = append(path, indexes...)
	}

	return path, nil
}

func parseIndexes(s string) ([]segment, error) {
	var out []segment

	for s != "" {
		if s[0] != '[' {
			return nil, fmt.Errorf("unexpected %q", s)
		}

		end := strings.IndexByte(s, ']')
		if end < 0 {
			return nil, fmt.Errorf("missing ] in %q", s)
		}

		n, err := strconv.Atoi(s[1:end])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid index %q", s[1:end])
		}

		out = append(out, segment{index: n})
		s = s[end+1:]
	}

	return out, nil
}

func validKey(k string) bool {
	if k == "" {
		return false
	}

	for _, r := range k {
		ok := r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}

	return true
}
