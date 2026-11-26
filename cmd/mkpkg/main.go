// Command mkpkg assembles the release tarball (docs/adr/0007). Reproducible: the timestamp
// comes from SOURCE_DATE_EPOCH, never the wall clock.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mkpkg:", err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", "", "tarball path to write")
	version := flag.String("version", "", "release version")
	binary := flag.String("binary", "", "stripped daemon binary")
	web := flag.String("web", "", "built web UI directory")
	files := flag.String("files", "", "directory copied verbatim into the tarball root/")
	install := flag.String("install", "", "install.sh")
	port80 := flag.String("port80", "", "port80.sh")
	pubkey := flag.String("pubkey", "", "minisign public key, shipped as minisign.pub (optional: unsigned build without it)")
	flag.Parse()
	for name, v := range map[string]string{"out": *out, "version": *version, "binary": *binary, "web": *web, "files": *files, "install": *install, "port80": *port80} {
		if v == "" {
			return fmt.Errorf("-%s is required", name)
		}
	}
	mtime, err := epoch(os.Getenv("SOURCE_DATE_EPOCH"))
	if err != nil {
		return err
	}
	in := Inputs{Version: *version}
	if in.Binary, err = os.ReadFile(*binary); err != nil {
		return err
	}
	if in.Install, err = readFile(*install, "install.sh"); err != nil {
		return err
	}
	if in.Port80, err = readFile(*port80, "port80.sh"); err != nil {
		return err
	}
	if *pubkey != "" {
		if in.Pubkey, err = os.ReadFile(*pubkey); err != nil {
			return err
		}
	}
	if in.Web, err = readTree(*web); err != nil {
		return err
	}
	if in.Files, err = readTree(*files); err != nil {
		return err
	}
	tarFiles := Assemble(in)
	var buf bytes.Buffer
	if err := Build(&buf, tarFiles, mtime); err != nil {
		return err
	}
	dir := filepath.Dir(*out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		return err
	}
	// Next to the tarball, same bytes as inside it: CI signs exactly what install.sh verifies.
	return os.WriteFile(filepath.Join(dir, "checksums.txt"), checksums(tarFiles), 0o644)
}

func epoch(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("SOURCE_DATE_EPOCH is not set")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH %q is not unix seconds", s)
	}
	return time.Unix(n, 0), nil
}

func readFile(path, name string) (File, error) {
	st, err := os.Stat(path)
	if err != nil {
		return File{}, err
	}
	body, err := os.ReadFile(path)
	return File{Name: name, Mode: st.Mode().Perm(), Body: body}, err
}

// readTree reads every regular file under dir; names are slash paths relative to dir.
func readTree(dir string) ([]File, error) {
	var out []File
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: not a regular file", p)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		f, err := readFile(p, filepath.ToSlash(rel))
		out = append(out, f)
		return err
	})
	return out, err
}

// checksums returns the body of the checksums.txt entry.
func checksums(files []File) []byte {
	for _, f := range files {
		if f.Name == "checksums.txt" {
			return f.Body
		}
	}
	return nil
}
