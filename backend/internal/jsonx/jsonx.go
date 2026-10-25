// Package jsonx encodes every API response and WebSocket message.
//
// It is encoding/json with one difference: a nil slice encodes as [] and a nil
// map as {}, never null. The OpenAPI document declares arrays non-nullable
// (ADR 0006 §4), so a null would break the generated frontend types, and a
// handler that forgets to initialise a slice is easy to write.
package jsonx

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
)

var options = json.JoinOptions(
	jsonv1.DefaultOptionsV1(),
	json.FormatNilSliceAsNull(false),
	json.FormatNilMapAsNull(false),
)

// Marshal encodes v like encoding/json.Marshal, but with nil collections empty.
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v, options)
}
