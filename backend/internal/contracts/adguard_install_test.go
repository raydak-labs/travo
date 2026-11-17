package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Both installation layouts must keep the file their init script reads. The
// package has its own keep list, but Travo's standalone installer does not.
func TestTravoKeepListPreservesAdGuardConfigs(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packaging", "openwrt", "files", "lib", "upgrade", "keep.d", "travo")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read keep list: %v", err)
	}
	for _, configPath := range []string{
		"/etc/adguardhome/adguardhome.yaml",
		"/opt/AdGuardHome/AdGuardHome.yaml",
	} {
		if !strings.Contains(string(raw), "\n"+configPath+"\n") {
			t.Errorf("%s does not preserve %s", path, configPath)
		}
	}
}
