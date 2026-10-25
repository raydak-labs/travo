package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

// The API is registered through the helpers in this file. Each operation's
// OpenAPI entry is generated from the Go types of its handler with Huma's
// schema registry, so there is no hand-written spec to drift (ADR 0006 §4).
//
// Requests are bound here rather than by huma.Register: Register is generic
// over every input/output pair and costs ~15 KB of code per operation, which
// put 3.5 MB on a binary that ships to router flash. The binder below is
// non-generic and covers what the API uses: string path and query parameters,
// a JSON body, the raw body, and the Fiber context for uploads and streams.

// OpenAPIPath is where the generated contract is served.
const OpenAPIPath = "/api/openapi.json"

// APIError is the error body of every endpoint: a human-readable message and,
// where a client must branch on the cause, a stable machine-readable code.
type APIError struct {
	status  int
	Message string `json:"error"`
	Code    string `json:"code,omitempty"`
}

func (e *APIError) Error() string  { return e.Message }
func (e *APIError) GetStatus() int { return e.status }

// statusError is an error with an HTTP status whose value is the response body.
type statusError interface {
	error
	GetStatus() int
}

func errorf(status int, msg string) error { return &APIError{status: status, Message: msg} }

// errCode builds an APIError carrying a stable code.
func errCode(status int, code, msg string) error {
	return &APIError{status: status, Message: msg, Code: code}
}

func badRequest(msg string) error { return errorf(http.StatusBadRequest, msg) }

// serverError reports err as a 500 with its message.
func serverError(err error) error { return errorf(http.StatusInternalServerError, err.Error()) }

// API collects the operations of the router and their OpenAPI document.
type API struct {
	router   fiber.Router
	doc      *huma.OpenAPI
	registry huma.Registry
	errorRef *huma.Schema
}

func init() {
	// Lists are documented as arrays, not "array or null": the frontend
	// treats them as arrays, and a handler that can answer null for an empty
	// list should answer [] instead.
	huma.DefaultArrayNullable = false
}

// NewAPI creates the API on app and serves its OpenAPI document at OpenAPIPath.
func NewAPI(app *fiber.App) *API {
	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	api := &API{
		router:   app,
		registry: registry,
		doc: &huma.OpenAPI{
			OpenAPI: "3.1.0",
			Info: &huma.Info{
				Title:       "OpenWRT Travel Router GUI API",
				Description: "REST API for managing an OpenWRT travel router",
				Version:     "1.0.0",
			},
			Components: &huma.Components{
				Schemas: registry,
				SecuritySchemes: map[string]*huma.SecurityScheme{
					"bearerAuth": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
				},
			},
			Security: []map[string][]string{{"bearerAuth": {}}},
		},
	}
	api.errorRef = registry.Schema(reflect.TypeFor[APIError](), true, "Error")
	app.Get(OpenAPIPath, func(c fiber.Ctx) error {
		data, err := api.doc.MarshalJSON()
		if err != nil {
			return err
		}
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.Send(data)
	})
	return api
}

// OpenAPI returns the generated document.
func (a *API) OpenAPI() *huma.OpenAPI { return a.doc }

type fiberCtxKey struct{}

// fiberCtx returns the request's Fiber context, for what the binder does not
// model: the client IP, the bearer token, uploads and streamed responses.
func fiberCtx(ctx context.Context) fiber.Ctx {
	c, _ := ctx.Value(fiberCtxKey{}).(fiber.Ctx)
	return c
}

// opt adjusts an operation's documentation before it is registered.
type opt func(*huma.Operation)

// public marks an operation as reachable without a token. auth.PublicPaths is
// what enforces it; this only documents it.
func public(op *huma.Operation) { op.Security = []map[string][]string{} }

// describe adds a longer description to an operation.
func describe(text string) opt { return func(op *huma.Operation) { op.Description = text } }

// created documents a 201 success.
func created(op *huma.Operation) { op.DefaultStatus = http.StatusCreated }

// upload documents a multipart/form-data body with one file field.
func upload(field string) opt {
	return func(op *huma.Operation) {
		op.RequestBody = &huma.RequestBody{
			Required: true,
			Content: map[string]*huma.MediaType{"multipart/form-data": {Schema: &huma.Schema{
				Type:       "object",
				Required:   []string{field},
				Properties: map[string]*huma.Schema{field: {Type: "string", Format: "binary"}},
			}}},
		}
	}
}

// formFlag adds an optional boolean field to an upload's multipart body.
func formFlag(name, doc string, def bool) opt {
	return func(op *huma.Operation) {
		op.RequestBody.Content["multipart/form-data"].Schema.Properties[name] = &huma.Schema{
			Type: "boolean", Description: doc, Default: def,
		}
	}
}

func get[I, O any](a *API, path, id, summary string, h func(context.Context, *I) (*O, error), opts ...opt) {
	route(a, http.MethodGet, path, id, summary, h, opts)
}

func post[I, O any](a *API, path, id, summary string, h func(context.Context, *I) (*O, error), opts ...opt) {
	route(a, http.MethodPost, path, id, summary, h, opts)
}

func put[I, O any](a *API, path, id, summary string, h func(context.Context, *I) (*O, error), opts ...opt) {
	route(a, http.MethodPut, path, id, summary, h, opts)
}

func del[I, O any](a *API, path, id, summary string, h func(context.Context, *I) (*O, error), opts ...opt) {
	route(a, http.MethodDelete, path, id, summary, h, opts)
}

// route documents and mounts one operation. Only this thin closure is generic.
func route[I, O any](a *API, method, path, id, summary string,
	h func(context.Context, *I) (*O, error), opts []opt,
) {
	in := reflect.TypeFor[I]()
	status := a.document(method, path, id, summary, in, reflect.TypeFor[O](), opts)
	bind := newBinder(in)
	a.router.Add([]string{method}, strings.NewReplacer("{", ":", "}", "").Replace(path), func(c fiber.Ctx) error {
		input := new(I)
		if err := bind(c, reflect.ValueOf(input).Elem()); err != nil {
			return writeError(c, err)
		}
		out, err := h(context.WithValue(c.Context(), fiberCtxKey{}, c), input)
		if err != nil {
			return writeError(c, err)
		}
		return writeOutput(c, status, out)
	})
}

// Body is a JSON request or response body.
type Body[T any] struct {
	Body T
}

// Stream is a response the handler writes itself through the Fiber context
// (file downloads, NDJSON streams).
type Stream struct {
	Write func()
}

func writeOutput(c fiber.Ctx, status int, out any) error {
	if s, ok := out.(*Stream); ok {
		s.Write()
		return nil
	}
	v := reflect.ValueOf(out)
	if v.IsNil() {
		return c.SendStatus(status)
	}
	return c.Status(status).JSON(v.Elem().FieldByName("Body").Interface())
}

func writeError(c fiber.Ctx, err error) error {
	var se statusError
	if errors.As(err, &se) {
		return c.Status(se.GetStatus()).JSON(se)
	}
	return c.Status(http.StatusInternalServerError).JSON(&APIError{Message: err.Error()})
}

type bindField struct {
	index []int
	kind  string // path, query, body, raw
	name  string
	ptr   bool
}

// newBinder precomputes how to fill an input struct from a request: `path` and
// `query` tagged string fields, Body (JSON, required unless it is a pointer)
// and RawBody (the bytes as sent).
func newBinder(t reflect.Type) func(fiber.Ctx, reflect.Value) error {
	var fields []bindField
	for i := range t.NumField() {
		f := t.Field(i)
		switch {
		case f.Tag.Get("path") != "":
			fields = append(fields, bindField{f.Index, "path", f.Tag.Get("path"), false})
		case f.Tag.Get("query") != "":
			fields = append(fields, bindField{f.Index, "query", f.Tag.Get("query"), false})
		case f.Name == "Body":
			fields = append(fields, bindField{f.Index, "body", "", f.Type.Kind() == reflect.Pointer})
		case f.Name == "RawBody":
			fields = append(fields, bindField{f.Index, "raw", "", false})
		}
	}
	// With a RawBody the handler decodes strictly itself (strictBody), so the
	// body is not decoded twice and the strict decoder's errors are the ones
	// reported.
	hasRaw := false
	for _, f := range fields {
		hasRaw = hasRaw || f.kind == "raw"
	}
	return func(c fiber.Ctx, v reflect.Value) error {
		for _, f := range fields {
			if f.kind == "body" && hasRaw {
				continue
			}
			dst := v.FieldByIndex(f.index)
			switch f.kind {
			case "path":
				dst.SetString(c.Params(f.name))
			case "query":
				dst.SetString(c.Query(f.name))
			case "raw":
				dst.SetBytes(append([]byte(nil), c.Body()...))
			case "body":
				body := c.Body()
				if len(strings.TrimSpace(string(body))) == 0 {
					if f.ptr {
						continue // an optional body
					}
					return badRequest(ErrInvalidRequestBody + ": request body is required")
				}
				target := dst.Addr().Interface()
				if f.ptr {
					dst.Set(reflect.New(dst.Type().Elem()))
					target = dst.Interface()
				}
				if err := json.Unmarshal(body, target); err != nil {
					return badRequest(ErrInvalidRequestBody + ": " + err.Error())
				}
			}
		}
		return nil
	}
}

// schema documents t, marking nullable references on the way (nullableRefs).
func (a *API) schema(t reflect.Type, hint string) *huma.Schema {
	s := a.registry.Schema(t, true, hint)
	a.nullableRefs(t, map[reflect.Type]bool{})
	return s
}

// nullableRefs documents a pointer-to-struct field without omitempty as
// "the struct or null", which is what encoding/json sends for a nil pointer.
// Huma's nullable tag does not apply to a referenced schema.
func (a *API) nullableRefs(t reflect.Type, seen map[reflect.Type]bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	parent := a.registry.Map()[huma.DefaultSchemaNamer(t, "")]
	for i := range t.NumField() {
		f := t.Field(i)
		a.nullableRefs(f.Type, seen)
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if parent == nil || name == "" || name == "-" || strings.Contains(opts, "omitempty") ||
			f.Type.Kind() != reflect.Pointer || f.Type.Elem().Kind() != reflect.Struct {
			continue
		}
		if prop := parent.Properties[name]; prop != nil && prop.Ref != "" {
			parent.Properties[name] = &huma.Schema{AnyOf: []*huma.Schema{{Ref: prop.Ref}, {Type: "null"}}}
		}
	}
}

// document adds the operation to the OpenAPI document and returns its success
// status.
func (a *API) document(method, path, id, summary string, in, out reflect.Type, opts []opt) int {
	op := &huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, DefaultStatus: http.StatusOK}
	for i := range in.NumField() {
		f := in.Field(i)
		switch {
		case f.Tag.Get("path") != "":
			op.Parameters = append(op.Parameters, &huma.Param{
				Name: f.Tag.Get("path"), In: "path", Required: true,
				Description: f.Tag.Get("doc"), Schema: &huma.Schema{Type: "string"},
			})
		case f.Tag.Get("query") != "":
			op.Parameters = append(op.Parameters, &huma.Param{
				Name: f.Tag.Get("query"), In: "query",
				Description: f.Tag.Get("doc"), Schema: &huma.Schema{Type: "string"},
			})
		case f.Name == "Body":
			t := f.Type
			if t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			op.RequestBody = &huma.RequestBody{
				Required: f.Type.Kind() != reflect.Pointer,
				Content: map[string]*huma.MediaType{"application/json": {
					Schema: a.schema(t, id+"Request"),
				}},
			}
		case f.Name == "RawBody":
			// Documents that unknown fields are rejected; TestWholeConfigPutsDecodeStrictly reads it.
			op.Extensions = map[string]any{"x-strict-body": true}
		}
	}
	for _, o := range opts {
		o(op)
	}
	if op.Responses == nil {
		op.Responses = map[string]*huma.Response{}
	}
	status := fmt.Sprint(op.DefaultStatus)
	if _, ok := op.Responses[status]; !ok {
		resp := &huma.Response{Description: http.StatusText(op.DefaultStatus)}
		if f, ok := out.FieldByName("Body"); ok {
			resp.Content = map[string]*huma.MediaType{"application/json": {
				Schema: a.schema(f.Type, id+"Response"),
			}}
		}
		op.Responses[status] = resp
	}
	op.Responses["default"] = &huma.Response{
		Description: "Error",
		Content:     map[string]*huma.MediaType{"application/json": {Schema: a.errorRef}},
	}
	a.doc.AddOperation(op)
	return op.DefaultStatus
}

// reply wraps v as a response body. It returns an error so a handler can end
// with `return reply(v)`.
//
//nolint:unparam // the nil error is the point
func reply[T any](v T) (*Body[T], error) { return &Body[T]{Body: v}, nil }

// replyOrFail returns v, or a 500 for err.
func replyOrFail[T any](v T, err error) (*Body[T], error) {
	if err != nil {
		return nil, serverError(err)
	}
	return reply(v)
}

// Status is the {"status":"ok"} acknowledgement.
type Status struct {
	Status string `json:"status" example:"ok"`
}

var statusOK = &Body[Status]{Body: Status{Status: "ok"}}

// okOrFail acknowledges success, or reports err as a 500.
func okOrFail(err error) (*Body[Status], error) {
	if err != nil {
		return nil, serverError(err)
	}
	return statusOK, nil
}

// none is an input without parameters or body.
type none struct{}

// strictBody is a request body that must not carry unknown fields: the
// endpoint persists the whole object, and a mismatched body would otherwise
// decode to zero values and overwrite the user's settings (decodeStrictJSON).
type strictBody[T any] struct {
	Body    T
	RawBody []byte
}

// decode replaces the permissively decoded body with a strict decode.
func (b *strictBody[T]) decode() error {
	var v T
	if err := decodeStrictJSON(b.RawBody, &v, ""); err != nil {
		return badRequest(ErrInvalidRequestBody + ": " + err.Error())
	}
	b.Body = v
	return nil
}

// strict adapts a handler of T to a strictBody[T] input.
func strict[T, O any](h func(context.Context, T) (*O, error)) func(context.Context, *strictBody[T]) (*O, error) {
	return func(ctx context.Context, in *strictBody[T]) (*O, error) {
		if err := in.decode(); err != nil {
			return nil, err
		}
		return h(ctx, in.Body)
	}
}
