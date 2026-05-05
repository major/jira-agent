package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteOption configures optional behavior for Write* functions.
type WriteOption func(*writeOptions)

type writeOptions struct {
	compact bool
}

// WithCompact enables compact output mode: strips null/empty fields from JSON,
// flattens single-key nested objects to dot-notation, and outputs JSON Lines
// for array data. Has no effect on CSV/TSV output.
func WithCompact(compact bool) WriteOption {
	return func(o *writeOptions) {
		o.compact = compact
	}
}

func applyOptions(opts []WriteOption) writeOptions {
	var o writeOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// compactStripEmpty recursively removes null values, empty strings, empty
// arrays, and empty maps from the data structure.
func compactStripEmpty(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return compactStripEmptyMap(v)
	case []any:
		items := make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, compactStripEmpty(item))
		}
		return items
	default:
		return value
	}
}

func compactStripEmptyMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, val := range value {
		if isEmptyValue(val) {
			continue
		}
		result[key] = compactStripEmpty(val)
	}
	return result
}

// isEmptyValue returns true for nil, empty strings, empty slices, and empty maps.
// Zero numeric values and false booleans are NOT considered empty.
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}

// compactFlatten flattens single-key nested map[string]any objects to
// dot-notation keys. Only one level of nesting is flattened: {"status":
// {"name": "Open"}} becomes {"status.name": "Open"}, but deeper nesting
// is preserved as-is.
func compactFlatten(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, val := range value {
		nested, ok := val.(map[string]any)
		if !ok || len(nested) != 1 {
			result[key] = val
			continue
		}
		// Single-key nested object: flatten to dot notation.
		for subKey, subVal := range nested {
			result[key+"."+subKey] = subVal
		}
	}
	return result
}

// applyCompact applies compact transformations to data: strip empties, then
// flatten single-key nested objects. Applied after compactJiraJSON so the
// Jira noise is already removed.
func applyCompact(value any) any {
	stripped := compactStripEmpty(value)
	switch v := stripped.(type) {
	case map[string]any:
		return compactFlatten(v)
	case []any:
		items := make([]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				items = append(items, compactFlatten(m))
			} else {
				items = append(items, item)
			}
		}
		return items
	default:
		return stripped
	}
}

// writeCompactJSONLines writes each element of an array as an individual JSON
// envelope on its own line (JSON Lines / NDJSON format). This allows LLMs to
// process results incrementally with minimal token overhead.
func writeCompactJSONLines(w io.Writer, items []any, errs []string, metadata Metadata, pretty bool) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	if len(items) == 0 {
		envelope := Envelope{
			Data:     []any{},
			Errors:   errs,
			Metadata: metadata,
		}
		return encoder.Encode(envelope)
	}
	for _, item := range items {
		envelope := Envelope{
			Data:     item,
			Errors:   errs,
			Metadata: metadata,
		}
		if err := encoder.Encode(envelope); err != nil {
			return fmt.Errorf("encoding compact JSON line: %w", err)
		}
	}
	return nil
}
