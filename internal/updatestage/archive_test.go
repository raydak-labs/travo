package updatestage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type testMember struct {
	name, body string
	kind       byte
}

func testPackage(version string, extra ...string) ([]testMember, string) {
	var members []testMember
	var paths []string
	var signed strings.Builder
	names := append(slices.Clone(required), extra...)
	for _, name := range names {
		if name == "checksums.txt" {
			continue
		}
		if strings.HasPrefix(name, "root/") {
			paths = append(paths, name[4:])
		}
	}
	slices.Sort(paths)
	for _, name := range names {
		if name == "checksums.txt" {
			continue
		}
		body := name
		switch name {
		case "VERSION":
			body = version + "\n"
		case "manifest.txt":
			body = strings.Join(paths, "\n") + "\n"
		}
		members = append(members, testMember{name, body, tar.TypeReg})
		fmt.Fprintf(&signed, "%x  ./%s\n", sha256.Sum256([]byte(body)), name)
	}
	return append(members, testMember{"checksums.txt", signed.String(), tar.TypeReg}), signed.String()
}

func tarBytes(t *testing.T, members []testMember) []byte {
	t.Helper()
	var raw bytes.Buffer
	w := tar.NewWriter(&raw)
	for _, member := range members {
		if err := w.WriteHeader(&tar.Header{Name: member.name, Typeflag: member.kind, Size: int64(len(member.body)), Mode: 0755}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(member.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}

func gzipBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	w := gzip.NewWriter(&compressed)
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

// A later release adds files the running stager has never heard of (#287): the signed
// manifest admits them under the package's own prefixes, never anywhere else.
func TestArchiveSignedAdditions(t *testing.T) {
	for _, tc := range []struct {
		name string
		ok   bool
	}{
		{"root/etc/hotplug.d/iface/60-travo-greenfield", true},
		{"root/usr/share/travo-greenfield/data.json", true},
		{webPrefix + "app.js", true},
		{webPrefix + "assets/app.js", false},
		{"root/sbin/travo-greenfield", false},
		{"root/etc", false},
		{"root/etc/../sbin/x", false},
		{"extra.sh", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			members, signed := testPackage("1.2.3", tc.name)
			dir := t.TempDir()
			archive := filepath.Join(dir, "archive")
			if err := os.WriteFile(archive, gzipBytes(t, tarBytes(t, members)), 0600); err != nil {
				t.Fatal(err)
			}
			err := extract(context.Background(), archive, filepath.Join(dir, "payload"), []byte(signed), "1.2.3")
			if (err == nil) != tc.ok {
				t.Fatalf("accepted=%v, want %v: %v", err == nil, tc.ok, err)
			}
		})
	}
}

func TestArchive(t *testing.T) {
	for _, name := range []string{"valid", "wrong version", "manifest mismatch", "missing member", "extra member", "duplicate", "traversal", "symlink", "oversize", "cancelled", "missing marker", "extra data", "extra gzip", "gnu metadata"} {
		t.Run(name, func(t *testing.T) {
			members, signed := testPackage("1.2.3")
			version := "1.2.3"
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "wrong version":
				version = "1.2.4"
			case "manifest mismatch":
				members[len(members)-1].body += "\n"
			case "missing member":
				members = members[1:]
			case "extra member":
				members = append(members, testMember{webPrefix + "extra", "x", tar.TypeReg})
			case "duplicate":
				members = append(members, members[0])
			case "traversal":
				members = append(members, testMember{webPrefix + "../evil", "x", tar.TypeReg})
			case "symlink":
				members = append(members, testMember{webPrefix + "link", "", tar.TypeSymlink})
			case "oversize":
				members[0].body = strings.Repeat("x", memberLimit+1)
			case "cancelled":
				cancel()
			}
			raw := tarBytes(t, members)
			switch name {
			case "missing marker":
				raw = raw[:len(raw)-1024]
			case "extra data":
				raw = append(raw, 0)
			case "gnu metadata":
				metadata := tarBytes(t, []testMember{{"metadata", "VERSION\x00", tar.TypeReg}})
				metadata[156] = tar.TypeGNULongName
				for i := 148; i < 156; i++ {
					metadata[i] = ' '
				}
				var sum int
				for _, b := range metadata[:512] {
					sum += int(b)
				}
				copy(metadata[148:156], fmt.Sprintf("%06o\x00 ", sum))
				raw = append(metadata[:1024], raw...)
			}
			blob := gzipBytes(t, raw)
			if name == "extra gzip" {
				blob = append(blob, gzipBytes(t, nil)...)
			}
			dir := t.TempDir()
			archive := filepath.Join(dir, "archive")
			if err := os.WriteFile(archive, blob, 0600); err != nil {
				t.Fatal(err)
			}
			err := extract(ctx, archive, filepath.Join(dir, "payload"), []byte(signed), version)
			if (err == nil) != (name == "valid") {
				t.Fatalf("unexpected verification result: %v", err)
			}
			if err == nil {
				info, err := os.Stat(filepath.Join(dir, "payload", "VERSION"))
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("payload mode: %v, %v", info, err)
				}
			}
		})
	}
}
