package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
)

// writeDelimited writes data as delimiter-separated rows with a header line.
// It accepts maps, slices of maps, structs (via JSON round-trip), or slices
// of any of those. Nested values are serialized as inline JSON strings.
func writeDelimited(w io.Writer, data any, delimiter rune) error {
	rows, err := normalizeToMaps(data)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	cw := csv.NewWriter(w)
	cw.Comma = delimiter
	defer cw.Flush()

	headers := extractHeaders(rows)
	if err := cw.Write(headers); err != nil {
		return err
	}

	for _, row := range rows {
		record := make([]string, len(headers))
		for i, h := range headers {
			record[i] = formatValue(row[h])
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}

	return cw.Error()
}

// normalizeToMaps converts arbitrary data into a uniform []map[string]any
// so the CSV writer has a consistent input shape.
func normalizeToMaps(data any) ([]map[string]any, error) {
	if data == nil {
		return nil, nil
	}

	// Single map.
	if m, ok := data.(map[string]any); ok {
		return []map[string]any{m}, nil
	}

	// Slice of maps.
	if ms, ok := data.([]map[string]any); ok {
		return ms, nil
	}

	// Slice of any (common from json.Unmarshal into []any).
	if slice, ok := data.([]any); ok {
		result := make([]map[string]any, 0, len(slice))
		for _, item := range slice {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
				continue
			}
			m, err := structToMap(item)
			if err != nil {
				return nil, fmt.Errorf("cannot convert slice element to map: %w", err)
			}
			result = append(result, m)
		}
		return result, nil
	}

	// Typed struct or pointer to struct: round-trip through JSON.
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		m, err := structToMap(data)
		if err != nil {
			return nil, err
		}
		return []map[string]any{m}, nil
	}

	// Typed slices of structs (e.g. []Issue).
	if v.Kind() == reflect.Slice {
		result := make([]map[string]any, 0, v.Len())
		for i := range v.Len() {
			m, err := structToMap(v.Index(i).Interface())
			if err != nil {
				return nil, fmt.Errorf("cannot convert slice element %d to map: %w", i, err)
			}
			result = append(result, m)
		}
		return result, nil
	}

	return nil, fmt.Errorf("CSV/TSV output requires a map, slice, or struct (got %T)", data)
}

// structToMap converts a struct to map[string]any via a JSON round-trip.
// Field names follow json struct tags, matching the JSON output columns.
func structToMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal to JSON for CSV conversion: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unmarshal to map for CSV conversion: %w", err)
	}
	return m, nil
}

// extractHeaders collects all unique keys across all rows and returns them
// sorted alphabetically for deterministic column order.
func extractHeaders(rows []map[string]any) []string {
	seen := make(map[string]struct{})
	for _, row := range rows {
		for k := range row {
			seen[k] = struct{}{}
		}
	}
	headers := make([]string, 0, len(seen))
	for k := range seen {
		headers = append(headers, k)
	}
	sort.Strings(headers)
	return headers
}

// formatValue converts an arbitrary value to its string representation for
// a CSV cell. Nested objects and arrays become inline JSON. Nil becomes empty.
func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		// JSON numbers decode as float64. Render whole numbers without
		// a decimal point so "42.0" becomes "42".
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	default:
		// Maps, slices, and other complex types: inline JSON.
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}
