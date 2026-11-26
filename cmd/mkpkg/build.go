package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"time"
)

// File is one regular file of the tarball; Name is the path inside it.
type File struct {
	Name string
	Mode fs.FileMode
	Body []byte
}

// Build writes files as a reproducible gzip'd tar: sorted by name, owner root, every
// timestamp mtime, no gzip file name. The same input gives the same bytes.
func Build(w io.Writer, files []File, mtime time.Time) error {
	sorted := append([]File(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Name == sorted[i-1].Name {
			return fmt.Errorf("duplicate entry %q", sorted[i].Name)
		}
	}
	mtime = mtime.Truncate(time.Second)
	zw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	zw.ModTime = mtime
	tw := tar.NewWriter(zw)
	for _, f := range sorted {
		h := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     f.Name,
			Mode:     int64(f.Mode.Perm()),
			Size:     int64(len(f.Body)),
			Uid:      0, Gid: 0,
			Uname: "root", Gname: "root",
			ModTime: mtime,
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := tw.Write(f.Body); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return zw.Close()
}

// Inputs are the already-read sources of one release. Web and Files names are relative to
// /www/travo-greenfield and / on the router.
type Inputs struct {
	Version string
	Binary  []byte
	Install File
	Port80  File
	// Pubkey is the minisign public key shipped as minisign.pub; nil for an unsigned build.
	Pubkey []byte
	Web    []File
	Files  []File
}

const (
	binaryTarget = "/usr/bin/travo-greenfield"
	webTarget    = "/www/travo-greenfield/"
)

// Assemble lays the sources out as the tarball (docs: issue #19 contract) and adds
// VERSION, manifest.txt, checksums.txt and, when in.Pubkey is set, minisign.pub. Binary and install.sh are always 0755.
func Assemble(in Inputs) []File {
	var root []File // Name = absolute target path
	root = append(root, File{Name: binaryTarget, Mode: 0o755, Body: in.Binary})
	for _, f := range in.Web {
		root = append(root, File{Name: webTarget + f.Name, Mode: f.Mode, Body: f.Body})
	}
	for _, f := range in.Files {
		root = append(root, File{Name: "/" + f.Name, Mode: f.Mode, Body: f.Body})
	}
	sort.Slice(root, func(i, j int) bool { return root[i].Name < root[j].Name })

	var manifest strings.Builder
	for _, f := range root {
		manifest.WriteString(f.Name + "\n")
	}
	install := in.Install
	install.Name, install.Mode = "install.sh", 0o755
	port80 := in.Port80
	port80.Name, port80.Mode = "port80.sh", 0o644
	files := []File{
		install,
		port80,
		{Name: "VERSION", Mode: 0o644, Body: []byte(in.Version + "\n")},
		{Name: "manifest.txt", Mode: 0o644, Body: []byte(manifest.String())},
	}
	if in.Pubkey != nil {
		files = append(files, File{Name: "minisign.pub", Mode: 0o644, Body: in.Pubkey})
	}
	for _, f := range root {
		files = append(files, File{Name: "root" + f.Name, Mode: f.Mode, Body: f.Body})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	var sums strings.Builder
	for _, f := range files {
		h := sha256.Sum256(f.Body)
		sums.WriteString(hex.EncodeToString(h[:]) + "  ./" + f.Name + "\n")
	}
	return append(files, File{Name: "checksums.txt", Mode: 0o644, Body: []byte(sums.String())})
}
