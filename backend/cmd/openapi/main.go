// Command openapi writes the backend's OpenAPI document to stdout, without a
// device or a running server. `make generate` uses it to regenerate the
// frontend's API types.
package main

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/api"
)

func main() {
	// Registration only documents and mounts handlers; no service is called,
	// so empty dependencies are enough.
	doc, err := api.SetupRoutes(fiber.New(), &api.Dependencies{}).OpenAPI().MarshalJSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(doc))
}
