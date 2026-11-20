package updatestage

import (
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
)

// The release tarball ships every packaging/files entry under root/; staging must accept each.
func TestRequiredCoversPackagingFiles(t *testing.T) {
	err := fs.WalkDir(os.DirFS("../../packaging/files"), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !slices.Contains(required, "root/"+name) {
			t.Errorf("packaging/files/%s is not in the staging allowlist", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range required {
		rest, ok := strings.CutPrefix(name, "root/")
		if !ok || rest == "usr/bin/travo-greenfield" || strings.HasPrefix(name, webPrefix) {
			continue
		}
		if _, err := os.Stat("../../packaging/files/" + rest); err != nil {
			t.Errorf("%s is required but not packaged: %v", name, err)
		}
	}
}
