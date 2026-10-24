package services

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A doc that points at a source file by name AND line number is a promise:
// "the guard path is at this line". A file split moves the line and the
// promise silently becomes false, and the reader who follows it lands on an
// unrelated line or past the end of the file.
//
// The existing docs gates check that a cited PATH exists. They cannot check
// that a cited LINE still holds what the doc says it holds, which is why the
// guard table in ADR 0003 drifted twice during the file-split refactor.
//
// This gate resolves every `file.ext:NN` reference in docs/ against the
// working tree and asserts the line exists. It does not check that the line
// still says the right thing - that is what a human or a reader does - but a
// reference past the end of a file is always wrong, and a reference to a
// renamed file is always wrong.
func TestDocFileLineReferencesResolve(t *testing.T) {
	repoRoot := "../../.."
	docsDir := filepath.Join(repoRoot, "docs")

	// A file reference with a line. The doc writes the file and the line in
	// two backtick groups, with a symbol name between them:
	//
	//	`services/wifi_service.go` (`guardPath` `:670`)
	//
	// or the line directly after the file: `services/wifi_service.go:670`.
	// The line part is optional; a reference without a line is covered by the
	// path gate instead.
	refRe := regexp.MustCompile("`([A-Za-z0-9_./-]+\\.(?:go|ts|tsx|sh|md|json|ya?ml|toml))`[^|]{0,60}?:([0-9]+)(?:-[0-9]+)?`?")

	// Roots a bare package-relative path may hang off.
	roots := []string{
		"", "backend/", "backend/internal/", "backend/cmd/", "backend/internal/services/",
		"backend/internal/api/", "backend/internal/auth/", "backend/internal/ws/",
		"backend/internal/uci/", "backend/internal/store/", "backend/internal/config/",
		"backend/internal/execx/", "backend/internal/ubus/", "backend/internal/models/",
		"frontend/src/", "shared/src/", "docs/", "docs/adr/", "docs/requirements/",
		"docs/plans/", "packaging/adguard/", "packaging/openwrt/",
	}

	docs := map[string]string{}
	err := filepath.WalkDir(docsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		// The archive is a frozen point-in-time record: it describes the
		// files as they were on the date it was written.
		if strings.Contains(path, string(filepath.Separator)+"_archive"+string(filepath.Separator)) {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		docs[path] = string(body)
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("no docs found; this gate would be vacuous")
	}

	checked, broken := 0, []string{}
	for doc, body := range docs {
		for _, m := range refRe.FindAllStringSubmatch(body, -1) {
			rel, lineStr := m[1], m[2]
			if lineStr == "" {
				continue // a path-only reference: the other gate covers it
			}
			if strings.Contains(rel, "*") {
				continue // a glob, not a file
			}
			line, convErr := strconv.Atoi(lineStr)
			if convErr != nil {
				t.Fatalf("%s: bad line number %q", doc, lineStr)
			}

			resolved := ""
			for _, root := range roots {
				candidate := filepath.Join(repoRoot, root+rel)
				if _, statErr := os.Stat(candidate); statErr == nil {
					resolved = candidate
					break
				}
			}
			if resolved == "" {
				broken = append(broken, fmt.Sprintf("%s cites %s:%s, which does not exist", doc, rel, lineStr))
				continue
			}
			checked++

			src, readErr := os.ReadFile(resolved)
			if readErr != nil {
				broken = append(broken, fmt.Sprintf("%s cites %s:%s, which could not be read", doc, rel, lineStr))
				continue
			}
			lines := strings.Split(string(src), "\n")
			if line < 1 || line > len(lines) {
				broken = append(broken, fmt.Sprintf(
					"%s cites %s:%s, but the file has only %d lines", doc, rel, lineStr, len(lines)))
			}
		}
	}

	if len(broken) > 0 {
		sortStrings(broken)
		for _, b := range broken {
			t.Error(b)
		}
	}
	if checked == 0 {
		t.Fatal("no file:line references were checked; this gate would be vacuous")
	}
	t.Logf("checked %d file:line references across %d docs", checked, len(docs))
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
