package cli

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

func writeCell(t *testing.T, path string, v uint32) {
	t.Helper()
	b := binary.BigEndian.AppendUint32(nil, v)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// keysFromCapture rebuilds the devicetree keys node from the verbatim listing: labels end
// in NUL, cells are big-endian u32, and EV_KEY nodes have no linux,input-type.
func keysFromCapture(t *testing.T) (string, []device.Button) {
	t.Helper()
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/buttons/devicetree-keys.tsv")
	if err != nil {
		t.Fatal(err)
	}
	want, err := mock.ParseButtonsTSV(b)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "compatible"), []byte("gpio-keys\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, k := range want {
		dir := filepath.Join(root, k.Node)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "label"), []byte(k.Label+"\x00"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeCell(t, filepath.Join(dir, "linux,code"), k.Code)
		if k.InputType != 1 {
			writeCell(t, filepath.Join(dir, "linux,input-type"), k.InputType)
		}
	}
	return root, want
}

func TestButtonsFromAXT1800Capture(t *testing.T) {
	root, want := keysFromCapture(t)
	got, err := (&Device{SysKeys: root}).Buttons(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got[1] != (device.Button{Node: "switch", Label: "switch", Code: 0x100, InputType: 5}) {
		t.Fatalf("switch = %+v", got[1])
	}
}

func TestButtonsWithoutKeysNode(t *testing.T) {
	got, err := (&Device{SysKeys: filepath.Join(t.TempDir(), "missing")}).Buttons(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want none", got, err)
	}
}

func TestButtonsRejectsMalformedCell(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "k"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "k", "linux,code"), []byte{1, 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Device{SysKeys: root}).Buttons(context.Background()); err == nil {
		t.Fatal("want an error for a 2-byte cell")
	}
}
