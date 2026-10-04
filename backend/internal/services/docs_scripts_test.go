package services

// Executable tests for the shell scripts under scripts/ and test/integration/.
//
// Every other claim about these scripts was a Go string-grep over the source.
// A grep cannot tell whether a script runs; it can only tell whether a line
// still says what it said yesterday. scripts/deploy-local.sh is the one script
// that CLEARS the crash guards from ADR 0003 section 2, so "it still mentions
// rm -f" was never evidence that it clears them on the right terms -- and it
// was clearing them before the restart they are supposed to follow.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// shellScripts returns every script the shellcheck gate covers, so the syntax
// gate below cannot fall behind the set `make shellcheck` checks.
func shellScripts(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{"scripts", "test/integration"} {
		matches, err := filepath.Glob(filepath.Join("..", "..", "..", dir, "*.sh"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		if len(matches) == 0 {
			t.Fatalf("no scripts found in %s — the glob is wrong", dir)
		}
		out = append(out, matches...)
	}
	return out
}

// TestShellScriptsParse runs each script through the shell parser. A syntax
// error in a device-mutating script is otherwise found by the operator, on the
// device, halfway through a run that has already restarted services.
func TestShellScriptsParse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell scripts")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not available: %v", err)
	}
	for _, script := range shellScripts(t) {
		t.Run(filepath.Base(script), func(t *testing.T) {
			// bash -n parses without executing a single line, and accepts the
			// #!/bin/sh scripts too.
			if out, err := exec.Command(bash, "-n", script).CombinedOutput(); err != nil {
				t.Fatalf("bash -n %s: %v\n%s", script, err, out)
			}
		})
	}
}

// deployRun is one recorded execution of scripts/deploy-local.sh against a
// stubbed device: every remote command the script asked for, in order.
type deployRun struct {
	exitCode int
	remote   []string
}

// guardRemovals returns the guard paths the run asked the device to delete.
//
// It PARSES the recorded command rather than matching an argv prefix. A prefix
// match is a spelling check, not a behaviour check: `rm -f -- /etc/trafo/x` and
// `sh -c 'rm -f /etc/trafo/x'` delete exactly what `rm -f /etc/trafo/x`
// deletes, so a detector that misses them misses the defect it exists to
// prevent (ADR 0003 section 1.3) for the cost of one character.
func (r deployRun) guardRemovals() []string {
	var out []string
	for _, cmd := range r.remote {
		out = append(out, guardPathsIn(cmd)...)
	}
	return out
}

// guardPathsIn returns every /etc/trafo* or /etc/travo* path the recorded
// command asks to remove.
func guardPathsIn(cmd string) []string {
	var out []string
	// `a; b`, `a && b` and `a | b` run b as its own command, so each simple
	// command has to be judged on its own argv.
	for _, seg := range splitCommands(cmd) {
		for _, p := range guardPathsInSimple(seg) {
			if isGuardPath(p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// guardPathsInSimple looks at one simple command (no `;`, `&&`, `|`).
func guardPathsInSimple(seg string) []string {
	var out []string
	words := splitWords(seg)
	for i := 0; i < len(words); i++ {
		switch words[i] {
		case "busybox", "env", "sudo", "nohup", "time", "timeout", "xargs":
			// The real command name follows the wrapper.
			continue
		case "sh", "bash", "dash", "ash", "zsh":
			out = append(out, rmPathsAfterShellC(words, i)...)
			return out
		case "rm":
			out = append(out, rmOperands(words[i+1:])...)
			return out
		}
	}
	return out
}

// rmPathsAfterShellC recurses into the string `sh -c '<command>'` runs. `-c` may
// carry other letters (`-lc`) and may be preceded by other flags, so the argv is
// scanned for it rather than assumed to sit at words[i+1].
func rmPathsAfterShellC(words []string, i int) []string {
	for j := i + 1; j < len(words); j++ {
		if !strings.HasPrefix(words[j], "-") {
			// `sh some-script.sh`: a file, not an inline command.
			return nil
		}
		if isShellCFlag(words[j]) {
			if j+1 < len(words) {
				return guardPathsIn(words[j+1])
			}
			return nil
		}
	}
	return nil
}

// isShellCFlag reports whether an `sh` option selects the inline command.
func isShellCFlag(opt string) bool {
	return len(opt) > 1 && strings.HasPrefix(opt, "-") && strings.HasSuffix(opt, "c")
}

// rmOperands splits rm's arguments into the paths it deletes: flags (including
// `--` and combined flags such as `-rf`) are dropped, everything else is a
// path. rm takes no option arguments, so any token starting with `-` before `--`
// is a flag.
func rmOperands(args []string) []string {
	var out []string
	for i, a := range args {
		if a == "--" {
			return append(out, args[i+1:]...)
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// isGuardPath reports whether p is a crash guard location. ADR 0003 keeps
// /etc/trafo and legacy /etc/travo; a glob such as /etc/trafo/* counts too,
// since a wildcard over the directory still removes every guard in it.
func isGuardPath(p string) bool {
	for _, dir := range []string{"/etc/trafo/", "/etc/travo/"} {
		if strings.HasPrefix(p, dir) {
			return true
		}
	}
	return false
}

// splitCommands splits a shell command on the operators that start a new simple
// command. Quote- and escape-aware: an operator inside "a; b" is data.
func splitCommands(cmd string) []string {
	var (
		out   []string
		cur   strings.Builder
		quote byte
	)
	flush := func() {
		if seg := strings.TrimSpace(cur.String()); seg != "" {
			out = append(out, seg)
		}
		cur.Reset()
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote != 0:
			cur.WriteByte(c)
			if c == quote {
				quote = 0
			} else if quote == '"' && c == '\\' && i+1 < len(cmd) {
				i++
				cur.WriteByte(cmd[i])
			}
		case c == '\'' || c == '"':
			quote = c
			cur.WriteByte(c)
		case c == '\\' && i+1 < len(cmd):
			i++
			cur.WriteByte(cmd[i])
		case c == ';':
			flush()
		case c == '|' || c == '&':
			// Consume the two-character forms `||`, `&&`, `|&` too.
			if i+1 < len(cmd) && (cmd[i+1] == '|' || cmd[i+1] == '&') {
				i++
			}
			flush()
		case c == '(' || c == ')':
			flush()
		case c == '\n':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
}

// splitWords splits a simple command into argv the way the remote shell does:
// honouring single quotes, double quotes and backslash escapes, and expanding
// nothing. The recorded string is the only evidence of what the device was asked
// to do, so the split has to be faithful rather than a regexp guess.
func splitWords(cmd string) []string {
	var (
		words []string
		cur   strings.Builder
		quote byte
		open  bool
	)
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case quote == '"':
			if c == '"' {
				quote = 0
			} else if c == '\\' && i+1 < len(cmd) {
				i++
				cur.WriteByte(cmd[i])
			} else {
				cur.WriteByte(c)
			}
		case c == '\'' || c == '"':
			quote = c
			open = true
		case c == '\\' && i+1 < len(cmd):
			i++
			cur.WriteByte(cmd[i])
			open = true
		case c == ' ' || c == '\t':
			if open {
				words = append(words, cur.String())
				cur.Reset()
				open = false
			}
		default:
			cur.WriteByte(c)
			open = true
		}
	}
	if open {
		words = append(words, cur.String())
	}
	return words
}

// sshStub records the remote command it is asked to run and answers pgrep
// according to serviceRunning. Nothing is executed and nothing is deleted: the
// guard paths are only recorded, so a regression cannot touch the developer's
// own /etc.
const sshStub = `#!/bin/sh
# ssh_cmd passes SSH_OPTS unquoted, so options arrive as separate arguments.
# Everything up to and including the first argument containing "@" is noise.
while [ $# -gt 0 ]; do
  case "$1" in
    *@*) shift; break ;;
    *) shift ;;
  esac
done
echo "$*" >> "SSH_LOG"
case "$*" in
  *pgrep*) [ "SERVICE_RUNNING" = "yes" ] ;;
  *) : ;;
esac
`

func runDeployLocal(t *testing.T, serviceRunning bool) deployRun {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatalf("mkdir stub dir: %v", err)
	}
	logPath := filepath.Join(dir, "remote.log")

	stub := strings.NewReplacer(
		"SSH_LOG", logPath,
		"SERVICE_RUNNING", yesNo(serviceRunning),
	).Replace(sshStub)
	for _, name := range []string{"ssh", "scp"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(stub), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}
	// sleep would add 8s plus 4x3s of wall clock per run for a wait that is not
	// what is under test.
	sleepStub := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(filepath.Join(bin, "sleep"), sleepStub, 0o755); err != nil {
		t.Fatalf("write sleep stub: %v", err)
	}

	// The stubs come first so ssh can never resolve to a real one, while the
	// system directories keep dirname and friends available; a mise shim
	// directory is deliberately absent so it cannot shadow the ssh stub.
	cmd := exec.Command("bash", filepath.Join("..", "..", "..", "scripts", "deploy-local.sh"),
		"--restart-only", "--ip", "192.0.2.1")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "TRAVO_INSECURE_SSH=1", "HOME=" + dir}
	out, err := cmd.CombinedOutput()

	run := deployRun{}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run deploy-local.sh: %v\n%s", err, out)
		}
		run.exitCode = exitErr.ExitCode()
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read recorded remote commands: %v\n%s", err, out)
	}
	run.remote = strings.Split(strings.TrimSpace(string(log)), "\n")
	return run
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// The retry path that clears crash guards must not erase the evidence of a
// failed deploy: ADR 0003 section 1.3 removes a guard only after the operation
// completes successfully. Clearing first, as this script did, left a device
// whose service never came back with no guard, no feature and nothing to grep
// for — the failure mode ADR 0003 exists to make discoverable.
func TestDeployLocalKeepsCrashGuardsWhenTheRestartFails(t *testing.T) {
	run := runDeployLocal(t, false)
	if run.exitCode == 0 {
		t.Fatal("deploy-local.sh reported success although the service never started; " +
			"a false OK is worse than a failure")
	}
	// Without this the assertion below would also pass if the script silently
	// did nothing at all, which is not the same as "it kept the guards".
	if !run.restartAttempted() {
		t.Fatal("deploy-local.sh never asked the device to restart travo, so the " +
			"guard assertion below is vacuous")
	}
	// The counterpart: the unmodified script must be able to get here with the
	// guards intact, i.e. the detector does not simply always fire.
	if removed := run.guardRemovals(); len(removed) > 0 {
		t.Errorf("a failed deploy cleared %d crash guard(s) (%v): the guards are the "+
			"only record that the device is mid-recovery, and ADR 0003 section 1.3 "+
			"removes them only after a successful operation", len(removed), removed)
	}
}

// The other half: the guards still have to be cleared on the successful retry
// path, or a stuck guard disables its feature forever with no recovery. The
// expected set is read from ADR 0003 section 2 rather than restated here, so a
// new row in the ADR cannot silently leave the script behind.
func TestDeployLocalClearsExactlyTheADR0003GuardsOnSuccess(t *testing.T) {
	adr, err := os.ReadFile(filepath.Join(
		"..", "..", "..", "docs", "adr", "0003-crash-guards-and-live-state.md"))
	if err != nil {
		t.Fatalf("read ADR 0003: %v", err)
	}
	adrGuards := guardPathsInADR(string(adr))
	if len(adrGuards) == 0 {
		t.Fatal("ADR 0003 section 2 lists no /etc/trafo guard paths: the table format " +
			"changed, so the comparison below would be vacuous")
	}

	run := runDeployLocal(t, true)
	if run.exitCode != 0 {
		t.Fatal("deploy-local.sh failed although the service started; the retry path " +
			"must work or ADR 0003's recovery promise is fiction")
	}
	removed := run.guardRemovals()
	if len(removed) == 0 {
		t.Fatal("a successful deploy cleared no crash guards at all: deploy-local.sh " +
			"is the documented recovery path for a stuck guard")
	}

	cleared := map[string]bool{}
	for _, p := range removed {
		cleared[p] = true
	}
	kept := map[string]bool{
		// A bounded retry counter, not a guard: only the device clears it, so a
		// broken saved network is not replayed (ADR 0003 section 2, last row).
		"autoreconnect-failcount": true,
		// Both survive the reboot they trigger; the record must outlive the deploy.
		"firmware-upgrade-in-progress": true,
		"factory-reset-in-progress":    true,
	}
	for _, dir := range []string{"/etc/trafo", "/etc/travo"} {
		for guard := range adrGuards {
			path := dir + "/" + guard
			if kept[guard] {
				if cleared[path] {
					t.Errorf("the recovery path clears %s, which ADR 0003 section 2 "+
						"deliberately keeps", path)
				}
				continue
			}
			if !cleared[path] {
				t.Errorf("the recovery path does not clear %s, so a stuck guard "+
					"permanently disables the feature it protects (ADR 0003 section 2)",
					path)
			}
		}
	}
}

// restartAttempted reports whether the run asked the device to restart travo.
func (r deployRun) restartAttempted() bool {
	for _, cmd := range r.remote {
		for _, seg := range splitCommands(cmd) {
			if strings.Contains(seg, "/etc/init.d/travo restart") {
				return true
			}
		}
	}
	return false
}

// Guard detection must key on what the command DOES, not on how it is spelled.
// The first three rows below are the defect ADR 0003 section 1.3 forbids, each
// spelled the way an otherwise-correct script would plausibly write it; the rest
// are non-guard deletions the detector must ignore, so it cannot pass by firing
// on everything.
func TestGuardRemovalsDetectsEverySpellingOfTheSameRemoval(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want []string
	}{
		{"plain", "rm -f /etc/trafo/mac-in-progress",
			[]string{"/etc/trafo/mac-in-progress"}},
		{"end of flags", "rm -f -- /etc/trafo/mac-in-progress",
			[]string{"/etc/trafo/mac-in-progress"}},
		{"quoted", `rm -f "/etc/trafo/mac-in-progress" '/etc/travo/mac-in-progress'`,
			[]string{"/etc/trafo/mac-in-progress", "/etc/travo/mac-in-progress"}},
		{"inner shell", `sh -c 'rm -f /etc/trafo/mac-in-progress'`,
			[]string{"/etc/trafo/mac-in-progress"}},
		{"inner shell, extra flags", `sh -lc 'rm -f -- /etc/trafo/mac-in-progress'`,
			[]string{"/etc/trafo/mac-in-progress"}},
		{"combined flags", "rm -rf /etc/trafo/mac-in-progress",
			[]string{"/etc/trafo/mac-in-progress"}},
		{"glob over the guard directory", "rm -f /etc/trafo/*-in-progress",
			[]string{"/etc/trafo/*-in-progress"}},
		{"after a chain", "echo ok && rm -f /etc/trafo/mac-in-progress || true",
			[]string{"/etc/trafo/mac-in-progress"}},
		{"not a guard", "rm -rf /www/travo.old /etc/uci-defaults/99-travel-gui-ports", nil},
		{"not a removal", "ls -l /etc/trafo/mac-in-progress", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := deployRun{remote: []string{tc.cmd}}
			got := run.guardRemovals()
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("guardRemovals() = %v, want %v for %q", got, tc.want, tc.cmd)
			}
		})
	}
}

// guardPathsInADR reads the guard basenames out of the ADR 0003 section 2 table.
func guardPathsInADR(adr string) map[string]bool {
	re := regexp.MustCompile(`/etc/trafo/([a-z0-9][a-z0-9-]*)`)
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(adr, -1) {
		out[m[1]] = true
	}
	return out
}
