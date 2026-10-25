package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidRequestBody prefixes every message about a malformed request.
const ErrInvalidRequestBody = "invalid request body"

// errInvalidJSON is the message for a body that is not JSON at all.
const errInvalidJSON = "invalid JSON"

// ErrUnknownField is returned by decodeStrictJSON when the request carries a
// field the endpoint does not know.
var ErrUnknownField = errors.New("unknown field")

// decodeStrictJSON decodes raw into dst, rejecting unknown fields. When wrapper
// is non-empty the object is unwrapped to that key first, so an endpoint can
// accept both a bare body and the envelope its GET returns.
func decodeStrictJSON(raw []byte, dst any, wrapper string) error {
	if wrapper != "" {
		var env map[string]json.RawMessage
		if err := json.Unmarshal(raw, &env); err != nil {
			return err
		}
		inner, ok := env[wrapper]
		if !ok {
			return fmt.Errorf("%w: missing %q", ErrUnknownField, wrapper)
		}
		raw = inner
	}
	// A literal `null` decodes into a non-pointer struct as a NO-OP: no error, and
	// the destination is left at its zero value. Since every caller of this helper
	// persists the decoded struct, `null` would have wiped the configuration
	// while the handler answered 200 — exactly the failure the strict binding
	// exists to prevent. encoding/json will not tell us, so reject it here.
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("body must be a JSON object, got null")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			// A field of the wrong JSON kind ("servers": "x" instead of a list).
			return fmt.Errorf("field %q: %w", typeErr.Field, err)
		}
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return errors.New(errInvalidJSON)
		}
		return err // json: unknown field "…"
	}
	// A second value in the body means the client is sending something we would
	// silently drop.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: unexpected trailing content", ErrUnknownField)
	}
	return nil
}
