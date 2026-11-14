package wifi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestReadOnlyPathsHaveNoWriteDependencies(t *testing.T) {
	// Join is a separate, transaction-backed path. Summary and scan stay read-only.
	for _, path := range []string{"service.go", "scan.go"} {
		t.Run(path, func(t *testing.T) {
			fset := token.NewFileSet()
			imports, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			deviceNames := map[string]bool{}
			for _, spec := range imports.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				switch importPath {
				case "github.com/blackdark-org/travo-greenfield/internal/txn",
					"github.com/blackdark-org/travo-greenfield/internal/uciwrite":
					t.Errorf("read-only package imports writer %s", importPath)
				case "github.com/blackdark-org/travo-greenfield/internal/device":
					name := "device"
					if spec.Name != nil {
						name = spec.Name.Name
					}
					if name == "." {
						t.Error("dot-importing device can expose UCI writers")
					} else {
						deviceNames[name] = true
					}
				}
			}

			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				qualifier, ok := selector.X.(*ast.Ident)
				if ok && deviceNames[qualifier.Name] {
					switch selector.Sel.Name {
					case "Device", "UCI", "Reloader":
						t.Errorf("read-only package uses writer device.%s", selector.Sel.Name)
					}
				}
				return true
			})
		})
	}
}

func TestJoinUsesDevicePorts(t *testing.T) {
	for _, path := range []string{"join.go", "join_plan.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range f.Imports {
			switch spec.Path.Value {
			case `"os/exec"`, `"syscall"`, `"github.com/blackdark-org/travo-greenfield/internal/device/cli"`:
				t.Errorf("%s accesses OpenWrt outside the Device ports (%s)", path, spec.Path.Value)
			}
		}
	}
}
