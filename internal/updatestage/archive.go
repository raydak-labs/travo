package updatestage

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const (
	expandedLimit = 24 << 20
	memberLimit   = 13 << 20
	maxMembers    = 256
	webPrefix     = "root/www/travo-greenfield/"
)

var required = []string{
	"VERSION", "checksums.txt", "install.sh", "manifest.txt", "minisign.pub", "port80.sh",
	"root/etc/hotplug.d/button/50-travo-greenfield", "root/etc/init.d/travo-greenfield",
	"root/etc/uci-defaults/90-travo-greenfield",
	"root/lib/upgrade/keep.d/travo-greenfield", "root/usr/bin/travo-greenfield",
	webPrefix + "index.html",
}

// A later release may add files under these prefixes; the signed manifest vouches for them
// (docs/adr/0032). Staging only extracts them: the update worker installs the binary and UI.
var signedPrefixes = []string{"root/etc/", "root/lib/", "root/usr/"}

var errArchive = errors.New("invalid release archive")

func allowed(name string) bool {
	if name == "" || path.Clean(name) != name || strings.ContainsAny(name, "\\\x00\r\n\t") {
		return false
	}
	if slices.Contains(required, name) {
		return true
	}
	for _, prefix := range signedPrefixes {
		if rest, ok := strings.CutPrefix(name, prefix); ok && rest != "" {
			return true
		}
	}
	// flat: the update worker copies the UI as one directory of regular files
	base, web := strings.CutPrefix(name, webPrefix)
	return web && base != "" && !strings.Contains(base, "/")
}

func checksums(manifest []byte) (map[string]string, error) {
	lines := strings.Split(string(manifest), "\n")
	if len(lines) < 2 || len(lines) > maxMembers || lines[len(lines)-1] != "" {
		return nil, errArchive
	}
	out := make(map[string]string, len(lines)-1)
	for _, line := range lines[:len(lines)-1] {
		hash, name, ok := strings.Cut(line, "  ./")
		digest, err := hex.DecodeString(hash)
		if !ok || err != nil || len(digest) != sha256.Size || hash != strings.ToLower(hash) ||
			!allowed(name) || name == "checksums.txt" || out[name] != "" {
			return nil, errArchive
		}
		out[name] = hash
	}
	for _, name := range required {
		if name != "checksums.txt" && out[name] == "" {
			return nil, errArchive
		}
	}
	return out, nil
}

type cancelReader struct {
	context.Context
	io.Reader
}

func (r cancelReader) Read(b []byte) (int, error) {
	if err := r.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(b)
}

func extract(ctx context.Context, archive, destination string, signed []byte, version string) error {
	hashes, err := checksums(signed)
	if err != nil {
		return err
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed := bufio.NewReader(cancelReader{ctx, file})
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	gzipReader.Multistream(false)
	expanded := &io.LimitedReader{R: cancelReader{ctx, gzipReader}, N: expandedLimit + 1}
	if err := extractMembers(expanded, destination, signed, hashes); err != nil {
		return err
	}
	var trailing [1]byte
	if n, err := io.ReadFull(expanded, trailing[:]); n != 0 || err != io.EOF || expanded.N == 0 {
		return errArchive
	}
	if _, err := compressed.ReadByte(); err != io.EOF {
		return errArchive
	}
	return verifyMetadata(destination, version, hashes)
}

func extractMembers(reader io.Reader, destination string, signed []byte, hashes map[string]string) error {
	seen := make(map[string]bool, len(hashes)+1)
	for {
		header, err := readHeader(reader)
		if err != nil {
			return err
		}
		if header == nil {
			break
		}
		name := header.Name
		if len(seen) == maxMembers || !allowed(name) || seen[name] || header.Typeflag != tar.TypeReg ||
			header.Size < 0 || header.Size > memberLimit || len(header.PAXRecords) != 0 {
			return errArchive
		}
		seen[name] = true
		member := &io.LimitedReader{R: reader, N: header.Size}
		if err := extractMember(member, destination, header, signed, hashes); err != nil {
			return err
		}
		padding := (512 - header.Size%512) % 512
		var block [512]byte
		if member.N != 0 || !readZeroes(reader, block[:padding]) {
			return errArchive
		}
	}
	if len(seen) != len(hashes)+1 || !seen["checksums.txt"] {
		return errArchive
	}
	return nil
}

// Read exactly one USTAR header. archive/tar normally consumes GNU and PAX
// metadata invisibly and accepts EOF without the package builder's end marker.
func readHeader(reader io.Reader) (*tar.Header, error) {
	var block [512]byte
	if _, err := io.ReadFull(reader, block[:]); err != nil {
		return nil, errArchive
	}
	if block == [512]byte{} {
		if !readZeroes(reader, block[:]) {
			return nil, errArchive
		}
		return nil, nil
	}
	if block[156] != tar.TypeReg {
		return nil, errArchive
	}
	header, err := tar.NewReader(bytes.NewReader(block[:])).Next()
	if err != nil || header.Format != tar.FormatUSTAR {
		return nil, errArchive
	}
	return header, nil
}

func readZeroes(reader io.Reader, buf []byte) bool {
	if _, err := io.ReadFull(reader, buf); err != nil {
		return false
	}
	for _, b := range buf {
		if b != 0 {
			return false
		}
	}
	return true
}

func extractMember(reader io.Reader, destination string, header *tar.Header, signed []byte, hashes map[string]string) error {
	name := header.Name
	if name == "checksums.txt" {
		if header.Size != int64(len(signed)) {
			return errArchive
		}
		raw, err := io.ReadAll(reader)
		if err != nil || !bytes.Equal(raw, signed) {
			return errArchive
		}
		return nil
	}
	if hashes[name] == "" {
		return errArchive
	}
	dest := filepath.Join(destination, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	hash := sha256.New()
	if err := writeBounded(dest, io.TeeReader(reader, hash), header.Size); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != hashes[name] {
		return errArchive
	}
	return nil
}

func verifyMetadata(destination, version string, hashes map[string]string) error {
	raw, err := os.ReadFile(filepath.Join(destination, "VERSION"))
	if err != nil || string(raw) != version+"\n" {
		return errArchive
	}
	var root []string
	for name := range hashes {
		if target, ok := strings.CutPrefix(name, "root"); ok {
			root = append(root, target)
		}
	}
	slices.Sort(root)
	raw, err = os.ReadFile(filepath.Join(destination, "manifest.txt"))
	if err != nil || string(raw) != strings.Join(root, "\n")+"\n" {
		return errArchive
	}
	return nil
}
