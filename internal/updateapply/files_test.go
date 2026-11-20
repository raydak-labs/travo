package updateapply

import (
	"io/fs"
	"os"
	"slices"
	"testing"
)

// install.sh writes every packaging/files entry into the manifest; an update must accept it.
func TestInstallerFilesMatchPackaging(t *testing.T) {
	var packaged []string
	err := fs.WalkDir(os.DirFS("../../packaging/files"), ".", func(name string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			packaged = append(packaged, "/"+name)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Sorted(slices.Values(installerFiles))
	if !slices.Equal(packaged, want) {
		t.Fatalf("packaging/files %v, installerFiles %v", packaged, want)
	}
}
