// Command openapi prints the daemon's OpenAPI 3.1 spec as JSON. It needs no device.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// specVersion keeps the committed spec stable; the real build version is not part of the contract.
const specVersion = "0.0.0"

// noTxns satisfies api.Txns: routes bind its methods at registration, and the spec
// needs registration only, never a call.
type noTxns struct{}

func (noTxns) Get(string) (txn.Status, error)      { return txn.Status{}, txn.ErrNotFound }
func (noTxns) Confirm(string) (txn.Status, error)  { return txn.Status{}, txn.ErrNotFound }
func (noTxns) Rollback(string) (txn.Status, error) { return txn.Status{}, txn.ErrNotFound }

// specDeps registers every route; the spec depends on registration, not on adapters.
func specDeps() api.Deps { return api.Deps{Version: specVersion, Txns: noTxns{}} }

func render(w io.Writer) error {
	_, a := api.New(specDeps())
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(a.OpenAPI())
}

func main() {
	if err := render(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "openapi:", err)
		os.Exit(1)
	}
}
