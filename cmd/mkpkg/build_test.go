package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"
)

var fixedTime = time.Unix(1700000000, 0)

// Deliberately unsorted: Build must sort.
func fixedFiles() []File {
	return []File{
		{Name: "root/usr/bin/tool", Mode: 0o755, Body: []byte("#!bin\n")},
		{Name: "VERSION", Mode: 0o644, Body: []byte("1.2.3\n")},
		{Name: "install.sh", Mode: 0o755, Body: []byte("echo hi\n")},
		{Name: "root/etc/config/x", Mode: 0o600, Body: nil},
	}
}

// goldenSHA256 pins the exact bytes for fixedFiles and fixedTime: any change to header
// fields, ordering or compression changes the tarball for every release.
const goldenSHA256 = "35c8620f301c6ef4eadfa428d9d1e3abbdb4f3a7389a9a5d3466eb63ace6ac7e"

func build(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Build(&buf, fixedFiles(), fixedTime); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBuildGolden(t *testing.T) {
	sum := sha256.Sum256(build(t))
	if got := hex.EncodeToString(sum[:]); got != goldenSHA256 {
		t.Fatalf("sha256 = %s, want %s", got, goldenSHA256)
	}
}

func TestBuildDeterministicAcrossInputOrder(t *testing.T) {
	files := fixedFiles()
	rev := make([]File, len(files))
	for i, f := range files {
		rev[len(files)-1-i] = f
	}
	var a, b bytes.Buffer
	if err := Build(&a, files, fixedTime); err != nil {
		t.Fatal(err)
	}
	if err := Build(&b, rev, fixedTime); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("output depends on input order")
	}
}

func TestBuildEntries(t *testing.T) {
	zr, err := gzip.NewReader(bytes.NewReader(build(t)))
	if err != nil {
		t.Fatal(err)
	}
	if !zr.ModTime.Equal(fixedTime) || zr.Name != "" {
		t.Fatalf("gzip header modtime=%v name=%q", zr.ModTime, zr.Name)
	}
	tr := tar.NewReader(zr)
	want := []struct {
		name string
		mode fs.FileMode
		body string
	}{
		{"VERSION", 0o644, "1.2.3\n"},
		{"install.sh", 0o755, "echo hi\n"},
		{"root/etc/config/x", 0o600, ""},
		{"root/usr/bin/tool", 0o755, "#!bin\n"},
	}
	for _, w := range want {
		h, err := tr.Next()
		if err != nil {
			t.Fatalf("%s: %v", w.name, err)
		}
		if h.Name != w.name {
			t.Fatalf("entry %q, want %q (sorted by name)", h.Name, w.name)
		}
		if h.FileInfo().Mode().Perm() != w.mode {
			t.Errorf("%s: mode %o, want %o", w.name, h.Mode, w.mode)
		}
		if h.Uid != 0 || h.Gid != 0 || h.Uname != "root" || h.Gname != "root" {
			t.Errorf("%s: owner %d/%d %s/%s", w.name, h.Uid, h.Gid, h.Uname, h.Gname)
		}
		if !h.ModTime.Equal(fixedTime) {
			t.Errorf("%s: mtime %v", w.name, h.ModTime)
		}
		body, _ := io.ReadAll(tr)
		if string(body) != w.body {
			t.Errorf("%s: body %q", w.name, body)
		}
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatalf("extra entries: %v", err)
	}
}

func TestBuildRejectsDuplicateNames(t *testing.T) {
	files := []File{{Name: "a", Mode: 0o644}, {Name: "a", Mode: 0o644}}
	if err := Build(io.Discard, files, fixedTime); err == nil {
		t.Fatal("want error for duplicate name")
	}
}

func TestAssemble(t *testing.T) {
	in := Inputs{
		Version: "1.2.3",
		Binary:  []byte("ELF"),
		Install: File{Name: "install.sh", Mode: 0o700, Body: []byte("sh\n")},
		Port80:  File{Body: []byte("helper\n")},
		Web:     []File{{Name: "index.html", Mode: 0o644, Body: []byte("<html>")}},
		Files:   []File{{Name: "etc/init.d/svc", Mode: 0o755, Body: []byte("rc\n")}},
	}
	files := Assemble(in)
	byName := map[string]File{}
	for _, f := range files {
		byName[f.Name] = f
	}
	if f := byName["install.sh"]; f.Mode != 0o755 {
		t.Errorf("install.sh mode %o, want 755", f.Mode)
	}
	if f := byName["port80.sh"]; f.Mode != 0o644 || string(f.Body) != "helper\n" {
		t.Errorf("port80.sh entry %+v", f)
	}
	if f := byName["root/usr/bin/travo-greenfield"]; f.Mode != 0o755 || string(f.Body) != "ELF" {
		t.Errorf("binary entry %+v", f)
	}
	if string(byName["VERSION"].Body) != "1.2.3\n" {
		t.Errorf("VERSION %q", byName["VERSION"].Body)
	}
	wantManifest := "/etc/init.d/svc\n/usr/bin/travo-greenfield\n/www/travo-greenfield/index.html\n"
	if got := string(byName["manifest.txt"].Body); got != wantManifest {
		t.Errorf("manifest %q, want %q", got, wantManifest)
	}
	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
	wantSums := sum("1.2.3\n") + "  ./VERSION\n" +
		sum("sh\n") + "  ./install.sh\n" +
		sum(wantManifest) + "  ./manifest.txt\n" +
		sum("helper\n") + "  ./port80.sh\n" +
		sum("rc\n") + "  ./root/etc/init.d/svc\n" +
		sum("ELF") + "  ./root/usr/bin/travo-greenfield\n" +
		sum("<html>") + "  ./root/www/travo-greenfield/index.html\n"
	if got := string(byName["checksums.txt"].Body); got != wantSums {
		t.Errorf("checksums\n%s\nwant\n%s", got, wantSums)
	}
	if _, ok := byName["root/etc/travo-greenfield/manifest.txt"]; ok {
		t.Error("manifest copy must be made by install.sh, not packed")
	}
}

func TestAssembleShipsPubkeyInChecksums(t *testing.T) {
	in := Inputs{Version: "1", Binary: []byte("ELF"), Install: File{Body: []byte("sh\n")}, Pubkey: []byte("untrusted comment: k\nRWQ\n")}
	byName := map[string]File{}
	for _, f := range Assemble(in) {
		byName[f.Name] = f
	}
	pub, ok := byName["minisign.pub"]
	if !ok || pub.Mode != 0o644 || string(pub.Body) != "untrusted comment: k\nRWQ\n" {
		t.Fatalf("minisign.pub entry %+v", pub)
	}
	h := sha256.Sum256(pub.Body)
	if want := hex.EncodeToString(h[:]) + "  ./minisign.pub\n"; !strings.Contains(string(byName["checksums.txt"].Body), want) {
		t.Errorf("checksums.txt lacks %q:\n%s", want, byName["checksums.txt"].Body)
	}
	if strings.Contains(string(byName["manifest.txt"].Body), "minisign.pub") {
		t.Error("minisign.pub must not be in the manifest (install.sh places it)")
	}
}

func TestAssembleWithoutPubkeyHasNoKeyEntry(t *testing.T) {
	for _, f := range Assemble(Inputs{Version: "1", Binary: []byte("ELF"), Install: File{Body: []byte("sh\n")}}) {
		if f.Name == "minisign.pub" {
			t.Fatal("unsigned build must not contain minisign.pub")
		}
	}
}
