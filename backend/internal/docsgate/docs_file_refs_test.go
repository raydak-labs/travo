package docsgate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Docs point at symbols, never at line numbers (AGENTS.md): a line number is a
// promise the next file split silently breaks.
func TestDocsCiteNoLineNumbers(t *testing.T) {
	refRe := regexp.MustCompile("`[A-Za-z0-9_./-]+\\.(?:go|ts|tsx|sh)(?::[0-9]+|`[^|`]{0,60}?`:[0-9]+)")
	docsDir := filepath.Join("..", "..", "..", "docs")
	scanned := 0
	err := filepath.WalkDir(docsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") ||
			strings.Contains(path, string(filepath.Separator)+"_archive"+string(filepath.Separator)) {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		for _, m := range refRe.FindAllString(string(body), -1) {
			t.Errorf("%s cites %s: name the symbol instead of a line", path, m)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs: %v", err)
	}
	if scanned == 0 {
		t.Fatal("no docs scanned; this gate would be vacuous")
	}
}
