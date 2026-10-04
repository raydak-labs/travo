package services

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docs/plans/README.md calls itself a "searchable catalog of all planning and
// historical design docs in this directory". It was missing 11 of 44 files,
// including the plan describing the most recent remediation effort, so two
// navigable sources of truth existed with no stated precedence — and a reader
// routing through the index would not find the current work.
//
// A hand-maintained index of a 44-file directory that is wrong about 11 entries
// is worse than no index, so the catalog is now checked mechanically.
func TestPlansIndexListsEveryPlan(t *testing.T) {
	const plansDir = "../../../docs/plans"
	const indexPath = "../../../docs/plans/README.md"

	entries, err := os.ReadDir(plansDir)
	if err != nil {
		t.Fatalf("read plans dir: %v", err)
	}

	body, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read plans index: %v", err)
	}
	index := string(body)

	listed := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" {
			continue
		}
		listed++
		if !strings.Contains(index, name) {
			t.Errorf("%s is not listed in docs/plans/README.md, which claims to be a "+
				"catalog of every plan in this directory", name)
		}
	}
	if listed < 40 {
		t.Fatalf("only %d plans found — the directory walk is probably broken", listed)
	}
}

// goPinRe reads the Go version each source of truth claims, so a divergence
// between mise, go.mod and CI cannot be discovered by a developer six months
// later the way the 2026-10-04 review found one: the remediation plan recorded a
// pin as "done" that had never been applied.
var (
	miseGoPinRe = regexp.MustCompile(`(?m)^\s*go\s*=\s*"([^"]+)"`)
	goModGoRe   = regexp.MustCompile(`(?m)^go\s+([0-9][0-9.]*)`)
)

func TestGoVersionPinsAgree(t *testing.T) {
	mise, err := os.ReadFile("../../../.mise.toml")
	if err != nil {
		t.Fatalf("read .mise.toml: %v", err)
	}
	goMod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	miseMatch := miseGoPinRe.FindSubmatch(mise)
	if miseMatch == nil {
		t.Fatalf("no go pin found in .mise.toml")
	}
	modMatch := goModGoRe.FindSubmatch(goMod)
	if modMatch == nil {
		t.Fatalf("no go directive found in go.mod")
	}

	miseVer := string(miseMatch[1])
	modVer := string(modMatch[1])
	if miseVer != modVer {
		t.Errorf("Go version pins disagree: .mise.toml pins %s but go.mod declares %s. "+
			"Developers and CI compile against different patch releases and nobody "+
			"notices. Align them, or document which one is authoritative and why.",
			miseVer, modVer)
	}

	// A third source exists and was not covered the first time this test was
	// written. Dockerfile.dev pins only the MINOR version, so it cannot drift on
	// the patch level, but it can drift on the minor one, which is the same class
	// of problem: the dev container compiling something CI never compiles.
	dockerfile, err := os.ReadFile("../../../Dockerfile.dev")
	if err != nil {
		t.Fatalf("read Dockerfile.dev: %v", err)
	}
	from := regexp.MustCompile(`FROM\s+golang:([0-9][0-9.]*)`).FindSubmatch(dockerfile)
	if from == nil {
		return // no golang base image; nothing to check
	}
	dockerMinor := strings.Split(string(from[1]), ".")[0]
	modMinor := strings.Split(modVer, ".")[0]
	if dockerMinor != modMinor {
		t.Errorf("Dockerfile.dev builds against Go %s but go.mod declares %s: the dev "+
			"container is a permanently divergent build environment.",
			string(from[1]), modVer)
	}
}

// TestReviewReportCitesOnlyExistingFiles keeps the review report honest about the
// one thing that made it hard to trust the previous one: completion marks that
// assert things about files which do not contain them.
func TestReviewReportIsPresentAndScoped(t *testing.T) {
	const report = "../../../2026-10-04-deep-code-review.md"

	body, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("the review report must exist alongside the work it describes: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"Method",
		"P0",
		"Architecture challenge",
		"Open questions",
		"device `192.168.1.1` was unreachable",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the review report is missing the section %q", want)
		}
	}

	// Every repo path the report cites must resolve to a real file. A review that
	// points at files that were renamed, or that never existed, is the same
	// failure mode as an ADR citing stale line numbers — and the 2026-09-26
	// review it follows is the example.
	//
	// Citations appear in both repo-root form ("backend/internal/auth/auth.go")
	// and package form ("services/vpn_service.go"), so each is resolved against a
	// small set of known roots. Paths that carry a glob are skipped: the report
	// legitimately cites patterns such as frontend/src/**.
	pathRe := regexp.MustCompile("`([A-Za-z0-9_./-]+\\.(?:go|ts|tsx|md|sh|json|json5|ya?ml|toml))(?::[0-9-]+)?`")
	roots := []string{
		"", "backend/", "backend/internal/", "backend/cmd/", "backend/internal/services/",
		"backend/internal/api/", "backend/internal/auth/", "frontend/src/", "shared/src/",
		"docs/", "docs/adr/", "docs/requirements/", "docs/plans/", "packaging/adguard/",
		"packaging/openwrt/",
	}
	checked, unresolved := 0, []string{}
	for _, m := range pathRe.FindAllStringSubmatch(text, -1) {
		rel := m[1]
		if strings.Contains(rel, "*") {
			continue
		}
		found := false
		for _, root := range roots {
			if _, err := os.Stat(filepath.Join("../../../", root+rel)); err == nil {
				found = true
				break
			}
		}
		if found {
			checked++
		} else {
			unresolved = append(unresolved, rel)
		}
	}
	if checked < 60 {
		t.Fatalf("only %d cited paths resolved (unresolved: %v) — either the report's "+
			"paths are wrong or the citation check is broken", checked, unresolved)
	}
}

// TestPublishedInstallInstructionsAreRunnable guards the two ways a security
// fix to the installer can break every user's install without any test noticing.
//
// The installer refuses to run without --password (it is the LuCI, SSH and Travo
// login for the device, so a published default is not acceptable), and the
// one-liner that people actually paste has no TTY, so any prompt is skipped. A
// documented or published one-liner that omits --password therefore dies at the
// first step -- and .github/workflows/release.yml publishes that text to the
// release page, which is not covered by any doc linter.
func TestPublishedInstallInstructionsAreRunnable(t *testing.T) {
	files := []string{
		"../../../README.md",
		"../../../docs/deployment.md",
		"../../../.github/workflows/release.yml",
		"../../../scripts/install.sh",
	}

	// A pipe into sh is the non-interactive shape: no TTY, so the password gate
	// fires. Lines are joined across backslash continuations first, because that
	// is how a long one-liner is written, and the trailing backslash is matched
	// explicitly rather than assumed away.
	pipeIntoSh := regexp.MustCompile(`\|\s*(\\\s*)?(sh|bash)\b`)

	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}

		var command strings.Builder
		for _, raw := range strings.Split(string(body), "\n") {
			line := strings.TrimSpace(raw)
			if command.Len() == 0 && (line == "" || strings.HasPrefix(line, "#")) {
				continue
			}
			continues := strings.HasSuffix(line, "\\")
			command.WriteString(strings.TrimSuffix(line, "\\"))
			command.WriteString(" ")
			if continues {
				continue
			}
			cmd := strings.TrimSpace(command.String())
			command.Reset()

			if !strings.Contains(cmd, "install.sh") || strings.Contains(cmd, "--uninstall") {
				continue
			}
			if !pipeIntoSh.MatchString(cmd) {
				continue // run directly, or piped into something other than a shell
			}
			if !strings.Contains(cmd, "--password") {
				t.Errorf("%s publishes a piped one-liner install with no --password:\n  %s\n"+
					"The installer has no default password, so this command dies before it "+
					"changes anything.", f, cmd)
			}
		}
	}
}

// TestInstallerHealthProbeUsesTheConfiguredPort is the same class of bug in the
// installer's own verification step: it hardcoded port 3000, but Travo defaults
// to port 80 (the installer moves LuCI to 8080 precisely so Travo can take 80),
// and with AdGuard installed 0.0.0.0:3000 belongs to AdGuard's web UI. Every
// install therefore ended in a die for a service that was running correctly.
func TestInstallerHealthProbeUsesTheConfiguredPort(t *testing.T) {
	body, err := os.ReadFile("../../../scripts/install.sh")
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	install := string(body)

	if strings.Contains(install, "TRAVO_PORT") {
		t.Error("scripts/install.sh still hardcodes a Travo port; read it from UCI " +
			"(travo.main.port, default 80) instead")
	}
	probe := regexp.MustCompile(`wget[^\n]*api/health`)
	if !probe.MatchString(install) {
		t.Fatal("install.sh no longer probes /api/health after starting the service; " +
			"the verify step is what stops it reporting success for a dead process")
	}
	// The probe must be bounded, or a filtered port makes each attempt block for
	// busybox wget's default read timeout and the retry loop is not a 15s bound.
	if !regexp.MustCompile(`wget[^\n]*-T\s+\d+[^\n]*api/health`).MatchString(install) {
		t.Error("the /api/health probe has no timeout (-T); a filtered or blackholed " +
			"port makes each attempt block and the install can hang for minutes")
	}
	// And it must read the port the service was actually configured with.
	if !strings.Contains(install, `uci -q get "${PKG_NAME}.main.port"`) {
		t.Error("install.sh does not read the port from travo.main.port")
	}
}
