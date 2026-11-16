package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// apiError is the one error shape: {error, code?}. code is stable for client branching.
type apiError struct {
	Status  int    `json:"-"`
	Message string `json:"error" doc:"Human-readable message; not for branching"`
	Code    string `json:"code,omitempty" doc:"Stable error code"`
}

func (e *apiError) Error() string  { return e.Message }
func (e *apiError) GetStatus() int { return e.Status }

func fail(status int, code, msg string) error {
	return &apiError{Status: status, Message: msg, Code: code}
}

func init() {
	huma.NewErrorWithContext = func(ctx huma.Context, status int, msg string, errs ...error) huma.StatusError {
		// JSON validation details can contain arbitrary property names from the import body.
		if ctx.Operation().OperationID == vpnImportOperation {
			return huma.NewError(status, "invalid profile request")
		}
		if id := ctx.Operation().OperationID; id == wifiJoinOperation || id == wifiProfileCreateOperation {
			return huma.NewError(status, "invalid WiFi join request")
		}
		return huma.NewError(status, msg, errs...)
	}
	// JSON keys are case-sensitive: `{"Password":...}` must not bind to `password`.
	huma.ValidateStrictCasing = true
	// Huma's own errors (validation, bad JSON) use the same shape. Only location and
	// message are echoed, never the offending value: it may be a password.
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		parts := []string{msg}
		for _, e := range errs {
			if d, ok := e.(huma.ErrorDetailer); ok {
				det := d.ErrorDetail()
				parts = append(parts, strings.TrimSpace(det.Location+": "+det.Message))
			}
		}
		code := ""
		if status == http.StatusUnprocessableEntity || status == http.StatusBadRequest {
			status, code = http.StatusBadRequest, "invalid_request" // invariants §4: bad input is 400
		}
		return &apiError{Status: status, Message: strings.Join(parts, "; "), Code: code}
	}
}

// writeError is for middleware, which runs before Huma can serialise an error.
func writeError(ctx huma.Context, status int, code, msg string) {
	ctx.SetHeader("Content-Type", "application/json")
	ctx.SetStatus(status)
	_ = json.NewEncoder(ctx.BodyWriter()).Encode(&apiError{Status: status, Message: msg, Code: code})
}

// jsonErrors turns the mux's plain-text 404/405 into the standard error shape.
type jsonErrors struct {
	http.ResponseWriter
	replaced bool
}

func (w *jsonErrors) WriteHeader(code int) {
	if (code == http.StatusNotFound || code == http.StatusMethodNotAllowed) &&
		strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
		w.replaced = true
		w.Header().Set("Content-Type", "application/json")
		w.ResponseWriter.WriteHeader(code)
		body := &apiError{Status: code, Message: strings.ToLower(http.StatusText(code))}
		if code == http.StatusNotFound {
			body.Code = "not_found"
		} else {
			body.Code = "method_not_allowed"
		}
		_ = json.NewEncoder(w.ResponseWriter).Encode(body)
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *jsonErrors) Write(b []byte) (int, error) {
	if w.replaced {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap keeps http.ResponseController (flush for SSE) working.
func (w *jsonErrors) Unwrap() http.ResponseWriter { return w.ResponseWriter }
