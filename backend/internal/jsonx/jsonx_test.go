package jsonx

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMarshalEncodesNilCollectionsEmpty(t *testing.T) {
	type resp struct {
		List []string         `json:"list"`
		Map  map[string]int   `json:"map"`
		Rows []map[string]any `json:"rows"`
	}
	got, err := Marshal(resp{})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"list":[],"map":{},"rows":[]}`; string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// Everything except nil collections must encode exactly like encoding/json,
// or the change would silently alter the wire format.
func TestMarshalOtherwiseMatchesEncodingJSON(t *testing.T) {
	type inner struct {
		N int `json:"n"`
	}
	type resp struct {
		Name     string        `json:"name"`
		Hidden   bool          `json:"hidden,omitempty"`
		Count    int           `json:"count,omitempty"`
		Note     string        `json:"note,omitempty"`
		Ptr      *inner        `json:"ptr,omitempty"`
		Nullable *inner        `json:"nullable"`
		HTML     string        `json:"html"`
		At       time.Time     `json:"at"`
		Dur      time.Duration `json:"dur"`
		Bytes    []byte        `json:"bytes"`
		Any      any           `json:"any"`
	}
	v := resp{
		Name: "a", HTML: "<b>&</b>", At: time.Date(2026, 10, 25, 1, 2, 3, 4, time.UTC),
		Dur: time.Second, Bytes: []byte("hi"), Any: map[string]any{"b": 1, "a": []int{1}},
	}
	want, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
