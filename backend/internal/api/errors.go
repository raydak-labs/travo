package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/gofiber/fiber/v3"
)

const (
	// Error messages for request validation
	ErrInvalidRequestBody = "invalid request body"
	ErrInvalidJSON        = "invalid JSON"
	ErrMissingField       = "missing required field"
	ErrInvalidValue       = "invalid value"
)

// RespondWithError sends a JSON error response with the given message and status.
func RespondWithError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

// RespondWithServerError sends a JSON error response with the error message and 500 status.
func RespondWithServerError(c fiber.Ctx, err error) error {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}

// RespondOK sends a JSON {"status":"ok"} response.
func RespondOK(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// ErrUnknownField is returned by BindStrictBodyConfig when the request carries a
// field the endpoint does not know.
var ErrUnknownField = errors.New("unknown field")

// BindStrictBodyConfig decodes a configuration request body and REJECTS unknown
// fields, unlike c.Bind().Body() which silently ignores them.
//
// Use it on every endpoint that persists a whole configuration object from the
// bound struct. With the permissive binder, a body that does not match the shape
// — a client that mirrors the GET response, a typo, a stale frontend — decodes to
// the zero value and the handler answers 200 after overwriting the user's
// settings with zeros. That is not hypothetical: PUT /wifi/band-switching returns
// {config, status} from GET but binds the bare struct, so a client that reads the
// documented GET shape and PUTs it back silently wipes every band-switching
// setting.
//
// A 400 saying which field is unknown is strictly better than a silent wipe, and
// it cannot break a correct client.
func BindStrictBodyConfig(c fiber.Ctx, dst any) error {
	return decodeStrictJSON(c.Body(), dst, "")
}

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
			return errors.New(ErrInvalidJSON)
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
