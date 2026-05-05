// Package jira provides typed utilities for working with Jira API data.
//
// Jira API responses deserialize into map[string]any because the schema is
// large, evolving, and includes unbounded custom fields. This package provides
// safe, zero-allocation extraction helpers that replace scattered inline type
// assertions throughout the codebase.
package jira

// Extract provides typed field extraction from map[string]any with safe
// zero-value defaults on missing or mistyped keys.
//
// Use NewExtract to wrap a map, then call typed accessors:
//
//	ext := jira.NewExtract(m)
//	name := ext.String("displayName")
//	id   := ext.Int64("id")
//	sub  := ext.Nested("fields").String("summary")
type Extract struct {
	M map[string]any
}

// NewExtract wraps a map for typed field access. A nil map is safe; all
// accessors return zero values.
func NewExtract(m map[string]any) Extract {
	return Extract{M: m}
}

// String returns the string value at key, or "" if missing or not a string.
func (e Extract) String(key string) string {
	if e.M == nil {
		return ""
	}
	v, _ := e.M[key].(string)
	return v
}

// Bool returns the bool value at key, or false if missing or not a bool.
func (e Extract) Bool(key string) bool {
	if e.M == nil {
		return false
	}
	v, _ := e.M[key].(bool)
	return v
}

// Int64 returns the int64 value at key. Handles float64 (JSON default),
// int64, and int. Returns 0 if missing or not a numeric type.
func (e Extract) Int64(key string) int64 {
	if e.M == nil {
		return 0
	}
	switch val := e.M[key].(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	default:
		return 0
	}
}

// Float64 returns the float64 value at key, or 0 if missing or not a float64.
func (e Extract) Float64(key string) float64 {
	if e.M == nil {
		return 0
	}
	v, _ := e.M[key].(float64)
	return v
}

// Map returns the nested map at key, or nil if missing or not a map.
func (e Extract) Map(key string) map[string]any {
	if e.M == nil {
		return nil
	}
	v, _ := e.M[key].(map[string]any)
	return v
}

// Slice returns the slice at key, or nil if missing or not a slice.
func (e Extract) Slice(key string) []any {
	if e.M == nil {
		return nil
	}
	v, _ := e.M[key].([]any)
	return v
}

// Nested returns an Extract wrapping the nested map at key. If the key is
// missing or not a map, the returned Extract has a nil map and all accessors
// return zero values. This enables safe chained access:
//
//	ext.Nested("fields").Nested("status").String("name")
func (e Extract) Nested(key string) Extract {
	return NewExtract(e.Map(key))
}
