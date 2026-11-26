// Command gentz writes zones.tsv: each IANA zone name with the POSIX TZ string OpenWrt
// stores in `system.@system[0].timezone`. The source is the TZif footer of every zone in
// Go's pinned $GOROOT/lib/time/zoneinfo.zip, so the table changes only with the Go version.
//
//	go run ./internal/syssettings/gentz > internal/syssettings/zones.tsv
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// regions are the canonical continent/ocean areas; legacy aliases (US/, Etc/, ...) stay out.
var regions = []string{"Africa/", "America/", "Antarctica/", "Arctic/", "Asia/", "Atlantic/", "Australia/", "Europe/", "Indian/", "Pacific/"}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gentz:", err)
		os.Exit(1)
	}
}

func run(w io.Writer) error {
	z, err := zip.OpenReader(filepath.Join(runtime.GOROOT(), "lib", "time", "zoneinfo.zip"))
	if err != nil {
		return err
	}
	defer z.Close()
	lines := []string{}
	for _, f := range z.File {
		if !canonical(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		tz, ok := footer(data)
		if !ok {
			return fmt.Errorf("%s: no TZif footer", f.Name)
		}
		lines = append(lines, f.Name+"\t"+tz)
	}
	sort.Strings(lines)
	_, err = fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

func canonical(name string) bool {
	for _, r := range regions {
		if strings.HasPrefix(name, r) {
			return true
		}
	}
	return false
}

// footer returns the POSIX TZ string between the last two newlines of a TZif v2+ file.
func footer(data []byte) (string, bool) {
	if !bytes.HasSuffix(data, []byte("\n")) {
		return "", false
	}
	body := data[:len(data)-1]
	i := bytes.LastIndexByte(body, '\n')
	if i < 0 {
		return "", false
	}
	tz := string(body[i+1:])
	return tz, tz != ""
}
