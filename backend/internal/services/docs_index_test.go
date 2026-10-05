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

// docs/plans/README.md calls itself a "searchable catalog of all planning and
// historical design docs in this directory". It was missing 11 of 44 files,
// including the plan describing the most recent remediation effort, so two
// navigable sources of truth existed with no stated precedence — and a reader
// routing through the index would not find the current work.
//
// A hand-maintained index of a directory that is wrong about its entries is
// worse than no index, so the catalog is checked mechanically. The floor is low
// on purpose: since ADR 0013, a shipped plan is deleted once its decisions are
// folded into an ADR, so this directory only holds live and normative plans. A
// sudden rise means plans are accumulating again and should be triaged.
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
	if listed < 5 {
		t.Fatalf("only %d plans found — the directory walk is probably broken, or live "+
			"and normative plans were deleted without folding their decisions into an ADR "+
			"(ADR 0013)", listed)
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
	published := []string{
		"../../../README.md",
		"../../../docs/guides/deployment.md",
		"../../../.github/workflows/release.yml",
	}
	install, err := os.ReadFile("../../../scripts/install.sh")
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	minPassword := minPasswordLength(t, install)
	reportOneLiners(t, minPassword, published,
		append(append([]string{}, published...), "../../../scripts/install.sh"))

	// The rule the published one-liners are checked against, on one-liners that
	// look right and are not. Each case is the flag being present but useless.
	t.Run("present but wrong", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			body string
		}{
			{
				name: "empty password",
				body: "```sh\nwget -O- https://x/install.sh | \\\n" +
					"  sh -s -- --password ''\n```\n",
			},
			{
				name: "password flag with no value",
				body: "```sh\nwget -O- https://x/install.sh | \\\n" +
					"  sh -s -- --password\n```\n",
			},
			{
				name: "password too short for the installer",
				body: "```sh\nwget -O- https://x/install.sh | \\\n" +
					"  sh -s -- --password 'abc'\n```\n",
			},
			{
				name: "one-liner only present as a comment",
				body: "```sh\n# wget -O- https://x/install.sh | \\\n" +
					"#   sh -s -- --password 'a-long-enough-password'\n```\n",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "published.md")
				if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
				problems, err := publishedOneLinerProblems(
					minPassword, []string{path}, []string{path})
				if err != nil {
					t.Fatalf("inspect fixture: %v", err)
				}
				if len(problems) == 0 {
					t.Errorf("a published one-liner that %s was accepted; the gate "+
						"cannot tell a runnable one-liner from a broken one", tc.name)
				}
			})
		}
	})

	t.Run("runnable one-liner passes", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "published.md")
		body := "```sh\nwget -O- https://x/install.sh | \\\n" +
			"  sh -s -- --password 'a-long-enough-password'\n```\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		reportOneLiners(t, minPassword, []string{path}, []string{path})
	})
}

// reportOneLiners runs the gate and turns its findings into test failures.
func reportOneLiners(
	t *testing.T,
	minPassword int,
	published []string,
	files []string,
) {
	t.Helper()
	problems, err := publishedOneLinerProblems(minPassword, published, files)
	if err != nil {
		t.Fatalf("inspect published one-liners: %v", err)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

// minPasswordLength reads the installer's own minimum password length so the
// published one-liners are checked against what the installer actually enforces
// rather than against a number copied into a test.
func minPasswordLength(t *testing.T, install []byte) int {
	t.Helper()
	match := regexp.MustCompile(`at least ([0-9]+) characters`).FindSubmatch(install)
	if match == nil {
		t.Fatal("install.sh no longer states a minimum password length; the gate " +
			"cannot tell a too-short published password from a usable one")
	}
	n, err := strconv.Atoi(string(match[1]))
	if err != nil || n < 1 {
		t.Fatalf("unusable minimum password length %q in install.sh", match[1])
	}
	return n
}

// passwordValueRe captures the value the installer is handed for --password,
// whether it is written --password 'x' or --password=x, so a bare flag or an
// empty one can be told from a real value.
var passwordValueRe = regexp.MustCompile(
	`--password(?:[=\s]+)("[^"]*"|'[^']*'|[^\s\\]*)?`)

// publishedOneLinerProblems is the gate over one set of files: every piped
// install one-liner in them must hand the installer a password it will accept,
// and every surface in published has to actually publish one. It returns the
// problems it found rather than failing, so the rules can be exercised directly.
//
// found is counted from live commands only. A commented-out example used to be
// counted, which let a surface satisfy "it publishes a one-liner" with an example
// nobody runs.
func publishedOneLinerProblems(
	minPassword int,
	published []string,
	files []string,
) ([]string, error) {
	var problems []string

	// A pipe into sh is the non-interactive shape: no TTY, so the password gate
	// fires. Lines are joined across backslash continuations first, because that
	// is how a long one-liner is written, and the trailing backslash is matched
	// explicitly rather than assumed away.
	pipeIntoSh := regexp.MustCompile(`\|\s*(\\\s*)?(sh|bash)\b`)

	found := map[string]int{}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}

		var command strings.Builder
		inComment := false
		for _, raw := range strings.Split(string(body), "\n") {
			line := strings.TrimSpace(raw)
			if command.Len() == 0 {
				// A comment line cannot be the start of a live command, and a
				// commented-out example must not satisfy the "publishes a
				// one-liner" requirement.
				inComment = strings.HasPrefix(line, "#")
				if inComment || line == "" {
					continue
				}
			}
			continues := strings.HasSuffix(line, "\\")
			command.WriteString(strings.TrimSuffix(line, "\\"))
			command.WriteString(" ")
			if continues {
				continue
			}
			cmd := strings.TrimSpace(command.String())
			command.Reset()
			commented := inComment
			inComment = false

			if !strings.Contains(cmd, "install.sh") || strings.Contains(cmd, "--uninstall") {
				continue
			}
			if !pipeIntoSh.MatchString(cmd) {
				continue // run directly, or piped into something other than a shell
			}
			if commented {
				continue // a commented-out example is not what a user pastes
			}
			found[f]++
			value := ""
			if pw := passwordValueRe.FindStringSubmatch(cmd); pw != nil {
				value = strings.Trim(pw[1], `"'`)
			}
			switch {
			case value == "":
				problems = append(problems, fmt.Sprintf(
					"%s publishes a piped one-liner install whose --password carries "+
						"no value:\n  %s\nThe installer has no default password, so "+
						"this command dies before it changes anything.", f, cmd))
			case len(value) < minPassword:
				problems = append(problems, fmt.Sprintf(
					"%s publishes a piped one-liner install with a %d-character "+
						"--password:\n  %s\nThe installer refuses anything under %d "+
						"characters, so this command dies before it changes anything.",
					f, len(value), cmd, minPassword))
			}
		}
	}

	// Each published surface has to actually publish one. release.yml writes the
	// release-page text, so it is the one nobody reads in the repo.
	for _, f := range published {
		if found[f] == 0 {
			problems = append(problems, fmt.Sprintf(
				"%s publishes no piped install one-liner any more; the gate cannot "+
					"check a one-liner that is not there, and this is a change to "+
					"what users are told to run", f))
		}
	}
	return problems, nil
}

// healthProbeRe finds the command that polls the service after starting it. The
// path may be spelled out or held in a variable, and the fetcher may be wget or
// curl, so only the fetch and the endpoint are required.
var healthProbeRe = regexp.MustCompile(
	`(?m)^[^\n]*?\b(wget|curl)\b[^\n]*api/health[^\n]*$`)

// literalPortRe finds a port written into a URL. Commit 2913502 hardcoded
// "127.0.0.1:3000" here, which is AdGuard's web UI: every install probed the
// wrong server and ended in a die for a service that was running fine.
//
// It matches any authority with digits after the colon, not just a host this
// test happens to have seen before, so "http://:3000/api/health" is caught too.
var literalPortRe = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s/"']*:[0-9]{2,5}`)

// probeFallbackRe finds a default expansion with a literal inside the probe
// itself, e.g. http://127.0.0.1:${PORT:-3000}/api/health. The variable is
// spelled correctly but the value is still a guess, so the URL is still a lie.
var probeFallbackRe = regexp.MustCompile(`:(?:=|-)?"?([0-9]{2,5})`)

// probePortVarRe captures the shell variable that supplies the port in a probe
// URL: "http://127.0.0.1:${_port}/api/health" yields _port. The gate follows
// that variable to wherever it is assigned, which is where a wrong default hides.
var probePortVarRe = regexp.MustCompile(
	`://[^\s/]*?:(?:\$\{([A-Za-z_][A-Za-z_0-9]*)[^}]*\}?` +
		`|\$([A-Za-z_][A-Za-z_0-9]*))[/"'\s;]`)

// varRefRe finds every shell variable referenced in a fragment.
var varRefRe = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z_0-9]*)\}?`)

// bareNumberRe finds a 2-5 digit token, i.e. something that can be a port.
var bareNumberRe = regexp.MustCompile(`\b[0-9]{2,5}\b`)

// ipv4Re matches an IPv4 address, which is a host and not a port: 127.0.0.1 in
// the probe URL must not be read as the port 127.
var ipv4Re = regexp.MustCompile(`\b[0-9]{1,3}(?:\.[0-9]{1,3}){3}\b`)

// configuredPortDefaults reads the port the packaged service actually starts on
// from the two places that state it: the uci config shipped in the package and
// the init script's config_get default. The gate must not carry its own copy of
// that number, or it would accept a fallback for a port the service no longer
// uses - which is exactly the bug it exists to catch.
func configuredPortDefaults(t *testing.T) map[string]bool {
	t.Helper()
	sources := []string{
		"../../../packaging/openwrt/files/etc/config/travo",
		"../../../packaging/openwrt/files/etc/init.d/travo",
	}
	defaults := map[string]bool{}
	for _, src := range sources {
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		port := regexp.MustCompile(`(?m)(?:option port|'port'|port main port) '?([0-9]{2,5})'?`)
		match := port.FindSubmatch(body)
		if match == nil {
			t.Fatalf("no default port found in %s; the gate can no longer tell a "+
				"sane fallback from AdGuard's 3000", src)
		}
		defaults[string(match[1])] = true
	}
	return defaults
}

// portChain is the resolved value flow of the probe's port: the statements of
// install.sh that can decide it, plus the variables the value travels through,
// plus the literal value of each of those variables when the statements spell
// it out.
type portChain struct {
	statements []string
	names      map[string]bool
	folded     map[string]string
	readsUCI   bool
}

// resolvePortChain walks the value flow of the probe's port variable, which is
// the rule install.sh itself follows: the port is read from uci into _port and
// only replaced if that read came back empty.
//
// The earlier version followed only statements that spelled the variable with a
// $ sigil, or that assigned the probe's variable by name. That missed two
// entirely ordinary ways of writing the same fallback, both of which shipped
// past it:
//
//	_fb="${TRAVO_PORT:-3000}"          # no sigil on the left, no $fb yet
//	[ -n "$_port" ] || _port="$_fb"    # the only line naming _port
//
//	_port="$(probe_fallback_port)"     # the literal is in a function body
//	probe_fallback_port() { printf %s 3000; }
//
// Both leave every line that carries the literal outside the walk. The rule
// used here is instead that the value flows through assignments, wherever the
// assignment sits and under whatever name, and through the bodies of functions
// those assignments call. Any depth of helper variable, any spelling of the
// default (literal, expansion, subshell, function, concatenation of two
// statements) is then reached by the same walk.
func resolvePortChain(lines []string, root string) portChain {
	names := map[string]bool{root: true}
	seen := map[int]bool{}
	funcOf, bodies := shellFunctions(lines)
	called := map[string]bool{}

	var statements []string
	for changed := true; changed; {
		changed = false
		for i, line := range lines {
			// A statement matters when it assigns to a variable the value flows
			// through, or when it sits in the body of a function the flow calls.
			relevant := seen[i] || (funcOf[i] != "" && called[funcOf[i]])
			if !relevant {
				for _, binding := range shellBindings(line) {
					if names[binding.name] {
						relevant = true
						break
					}
				}
			}
			if !relevant {
				continue
			}
			if !seen[i] {
				seen[i] = true
				statements = append(statements, line)
			}
			for _, ref := range varRefRe.FindAllStringSubmatch(line, -1) {
				if !names[ref[1]] {
					names[ref[1]] = true
					changed = true
				}
			}
			for _, word := range wordRe.FindAllString(line, -1) {
				if _, ok := bodies[word]; ok && !called[word] {
					called[word] = true
					changed = true
				}
			}
		}
	}

	chain := portChain{
		statements: statements,
		names:      names,
		folded:     map[string]string{},
	}
	values := map[string]string{}
	for _, line := range lines {
		for _, binding := range shellBindings(line) {
			// Walking the assignments in order lets a variable be assembled
			// from parts that are themselves literals: _hi="30" then
			// _lo="00" then _p="${_hi}${_lo}" folds to 3000.
			if value, ok := foldPortLiteral(binding.value, values); ok {
				values[binding.name] = value
				continue
			}
			delete(values, binding.name)
		}
	}
	for _, statement := range statements {
		if strings.Contains(statement, "uci") && strings.Contains(statement, ".main.port") {
			chain.readsUCI = true
		}
	}
	for name, value := range values {
		if names[name] {
			chain.folded[name] = value
		}
	}
	return chain
}

// binding is one assignment in a shell statement: the variable written to and
// the value written. `read` is deliberately absent - a value read from stdin or
// /dev/tty is not a guess the script invented.
type binding struct {
	name  string
	value string
}

// shellBindings finds the assignments in a statement. The variable is the word
// immediately left of an = or +=, optionally behind local/export/readonly/
// declare/typeset/printf -v, so `local _port=3000`, `_port="${P:-3000}"` and
// `[ -z "$_port" ] && _port=3000` all yield the same thing. A comparison like
// [ "$x" = 1 ] does not: its = is preceded by a quote.
var shellBindingRe = regexp.MustCompile(
	`(?:^|[\s;|&(])(?:local|export|readonly|declare|typeset|printf\s+-v)?\s*` +
		`([A-Za-z_][A-Za-z_0-9]*)\s*(\+?)=([^\n]*)`)

// shellBindings returns every assignment in one logical statement.
func shellBindings(statement string) []binding {
	var out []binding
	for _, match := range shellBindingRe.FindAllStringSubmatch(statement, -1) {
		out = append(out, binding{
			name:  match[1],
			value: strings.TrimSpace(match[3]),
		})
	}
	return out
}

// wordRe finds every bare word, used to spot a call to a function defined in
// the same script.
var wordRe = regexp.MustCompile(`[A-Za-z_][A-Za-z_0-9]*`)

// shellFunctions maps each function defined in the script to the lines of its
// body - the definition line included, since `error() { ...; }` is one line -
// and every line to the function that encloses it. A fallback written in a
// helper is still the fallback the probe uses.
func shellFunctions(lines []string) (map[int]string, map[string][]int) {
	def := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z_0-9]*)\s*\(\s*\)\s*\{`)
	funcOf := map[int]string{}
	bodies := map[string][]int{}
	current := ""
	for i, line := range lines {
		if match := def.FindStringSubmatch(line); match != nil {
			current = match[1]
			funcOf[i] = current
			bodies[current] = append(bodies[current], i)
			if strings.Contains(line, "}") {
				current = ""
			}
			continue
		}
		if current == "" {
			continue
		}
		if strings.TrimSpace(line) == "}" {
			current = ""
			continue
		}
		funcOf[i] = current
		bodies[current] = append(bodies[current], i)
	}
	return funcOf, bodies
}

// commandSubstRe spots a value the script asks something else for. Such a
// value is not a guess the script invented, so it is left undecidable.
var commandSubstRe = regexp.MustCompile("`\\$\\(|`|<\\(|>\\(")

// foldPortLiteral resolves an assignment value to the literal digits it spells,
// following variables that are themselves literal. It reports false when the
// value comes from a command, a test, the configuration, or an unresolved
// variable - that is, whenever the statement is not itself the guess.
func foldPortLiteral(value string, values map[string]string) (string, bool) {
	trimmed := strings.Trim(value, "\"'` ")
	if trimmed == "" {
		return "", true
	}
	if commandSubstRe.MatchString(trimmed) {
		return "", false
	}
	var out strings.Builder
	rest := trimmed
	for {
		loc := varRefRe.FindStringIndex(rest)
		if loc == nil {
			break
		}
		if !isDigits(rest[:loc[0]]) {
			return "", false
		}
		out.WriteString(rest[:loc[0]])
		name := strings.Trim(strings.TrimPrefix(rest[loc[0]:loc[1]], "$"), "{}")
		resolved, ok := values[name]
		if !ok {
			return "", false
		}
		out.WriteString(resolved)
		rest = rest[loc[1]:]
	}
	if !isDigits(rest) {
		return "", false
	}
	out.WriteString(rest)
	return out.String(), true
}

// isDigits reports whether s is empty or only decimal digits.
func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// logicalStatements joins line continuations and drops comments, so the walk
// sees the statements the shell sees and not the prose around them.
func logicalStatements(script string) []string {
	var out []string
	var current string
	for _, line := range strings.Split(script, "\n") {
		code := stripShellComment(line)
		trimmed := strings.TrimRight(code, " \t")
		if strings.HasSuffix(trimmed, "\\") {
			current += strings.TrimSuffix(trimmed, "\\") + " "
			continue
		}
		out = append(out, strings.TrimSpace(current+trimmed))
		current = ""
	}
	if strings.TrimSpace(current) != "" {
		out = append(out, strings.TrimSpace(current))
	}
	return out
}

// stripShellComment drops a trailing comment. ${#var} is not a comment, so a
// # directly after a ${ is left alone.
func stripShellComment(line string) string {
	depth := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '#':
			if depth == 0 {
				return line[:i]
			}
		}
	}
	return line
}

// TestInstallerHealthProbeUsesTheConfiguredPort is the same class of bug in the
// installer's own verification step: it hardcoded port 3000, but Travo defaults
// to port 80 (the installer moves LuCI to 8080 precisely so Travo can take 80),
// and with AdGuard installed 0.0.0.0:3000 belongs to AdGuard's web UI. Every
// install therefore ended in a die for a service that was running correctly.
//
// The assertions are about the port the probe actually dials, not about the
// spelling of the line. Two earlier checks failed both ways: one flagged the
// variable name TRAVO_PORT, which a correct fix may well use, and the other
// required wget's -T to appear before the URL, so a correct script that puts the
// flags last failed. Neither could catch the regression it described, because
// a hardcoded 3000 satisfies both.
//
// A third version inspected only the FIRST probe line, so it passed on a script
// that still probed 127.0.0.1:3000 somewhere else. This one checks every probe,
// and - the part that matters - it follows the probe's port variable to where it
// is assigned. Reinstating the real 2913502 defect
//
//	_port="$(uci -q get "${PKG_NAME}.main.port" 2>/dev/null || true)"
//	[ -n "$_port" ] || _port="3000"
//
// satisfies every earlier check in this file (the line interpolates a variable,
// carries -T, and the uci read is still present) while every install probes
// AdGuard's web UI and dies. So the fallback is checked against the port the
// packaged service really starts on, read out of the package rather than
// restated here.
//
// The version after that followed the probe's port variable only through lines
// that spelled the name with a $ sigil or assigned the probe's variable
// directly. That still hid four ordinary spellings of the same fallback - one
// computed in a subshell into a helper variable, one behind a function, one
// split across two variables and concatenated, and one written as a conditional
// expansion of another variable - all of which executed and all of which passed.
// The gate now walks the whole value flow (see resolvePortChain) and folds a
// value assembled from literals, so the spelling does not decide whether the
// fallback is seen. What it still cannot do is listed in the report for this
// change: a port computed by an external command, or a default carried in a
// file the script reads rather than in an assignment.
func TestInstallerHealthProbeUsesTheConfiguredPort(t *testing.T) {
	body, err := os.ReadFile("../../../scripts/install.sh")
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	install := string(body)
	lines := logicalStatements(install)
	defaults := configuredPortDefaults(t)

	found := healthProbeRe.FindAllStringSubmatch(install, -1)
	if len(found) == 0 {
		t.Fatal("install.sh no longer probes /api/health after starting the service; " +
			"the verify step is what stops it reporting success for a dead process")
	}

	uciPortRead := false
	activeProbes := 0
	for _, match := range found {
		// The whole line is the probe, so a flag written after the URL is still
		// part of it; a commented-out example is not a probe.
		probe := strings.TrimSpace(match[0])
		fetcher := match[1]
		if regexp.MustCompile(`^#`).MatchString(probe) {
			continue
		}
		activeProbes++
		if literal := literalPortRe.FindString(probe); literal != "" {
			t.Errorf("the /api/health probe dials a literal %s: the port must come "+
				"from the UCI the service was configured with (travo.main.port), "+
				"because 0.0.0.0:3000 is AdGuard's web UI and polling it fails "+
				"every install.\n  %s", literal, probe)
			continue
		}
		// A default expansion is only acceptable if it is the port the packaged
		// service actually starts on; anything else is a guess in disguise.
		for _, fallback := range probeFallbackRe.FindAllStringSubmatch(probe, -1) {
			if defaults[fallback[1]] {
				continue
			}
			t.Errorf("the /api/health probe defaults the port to %s: a correctly "+
				"named variable whose default is somebody else's port probes "+
				"AdGuard and dies.\n  %s", fallback[1], probe)
		}
		// The probe must be bounded, or a filtered port makes each attempt block
		// for the fetcher's default read timeout and the retry loop is not a short
		// bound. Flag order is irrelevant; wget spells it -T, curl --max-time.
		if fetcher == "wget" {
			if !regexp.MustCompile(`(^|\s)-T\s+[0-9]+`).MatchString(probe) {
				t.Errorf("the wget /api/health probe has no timeout (-T): a filtered "+
					"or blackholed port makes each attempt block and the install "+
					"can hang.\n  %s", probe)
			}
		} else if !strings.Contains(probe, "--max-time") {
			t.Errorf("the curl /api/health probe has no --max-time: a filtered or "+
				"blackholed port makes each attempt block and the install can "+
				"hang.\n  %s", probe)
		}

		portVar := probePortVarRe.FindStringSubmatch(probe)
		if portVar == nil || (portVar[1] == "" && portVar[2] == "") {
			t.Errorf("the /api/health probe does not dial a port taken from the "+
				"configuration (%s): the port has to be the one the service was "+
				"started on", probe)
			continue
		}
		name := portVar[1] + portVar[2]
		chain := resolvePortChain(lines, name)
		uciPortRead = uciPortRead || chain.readsUCI
		// A value spelled out in digits, however it was assembled across
		// statements, is the guess the gate is looking for.
		for varName, folded := range chain.folded {
			if !bareNumberRe.MatchString(folded) || defaults[folded] {
				continue
			}
			t.Errorf("the port the /api/health probe dials can come from the "+
				"literal %s: %s is assigned only from literals, so it is a "+
				"hardcoded guess and not the configured port.\n  %s is set to %s",
				folded, varName, varName, folded)
		}
		for _, source := range chain.statements {
			numbers := bareNumberRe.FindAllString(
				ipv4Re.ReplaceAllString(source, "HOST"), -1)
			for _, number := range numbers {
				if defaults[number] {
					continue
				}
				t.Errorf("the port the /api/health probe dials can come from the "+
					"literal %s: it must fall back only to the port the packaged "+
					"service is configured with (travo.main.port), because 3000 is "+
					"AdGuard's web UI and polling it fails every install.\n  %s",
					number, strings.TrimSpace(source))
			}
		}
	}

	// And the port it interpolates must come from the configured one. Checked
	// against the lines that decide that port, not against the whole script.
	if activeProbes == 0 {
		t.Fatal("every /api/health probe in install.sh is commented out: the " +
			"installer no longer verifies the service it just started")
	}
	if !uciPortRead {
		t.Error("install.sh does not read the port from travo.main.port (uci get " +
			"…main.port); a probe without it is polling a guess")
	}
}
