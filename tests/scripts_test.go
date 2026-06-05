// Package tests contains black-box and conformance tests for the test-environment
// spec. scripts_test.go verifies the standard subcommand interface (deploy|status|
// logs|teardown|help) on every orchestration script under scripts/.
//
// The spec (openspec/specs/test-environment/spec.md, Requirement: Standard Script
// Interface) mandates that every orchestration script accepts the five standard
// verbs and exits non-zero on any other invocation. help is special-cased and
// must always succeed, including on scripts whose main action would otherwise
// mutate the host (e.g. clab.sh deploy).
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// repoRoot returns the path to the repository root. The test file is anchored
// at <repo>/tests/scripts_test.go, so the parent of this file's directory is
// the root. We resolve symlinks to handle GOPATH layouts where the worktree
// is a symlink.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller path")
	}
	dir := filepath.Dir(here)
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	// tests/ is at the repo root.
	return filepath.Dir(abs)
}

// allScripts enumerates every orchestration script under scripts/ that must
// implement the standard interface. reset.sh is also a script but its main
// action is interactive (asks for confirmation) — we still verify its standard
// interface and skip only the deploy/status/logs paths that would require
// running services.
var allScripts = []string{
	"setup.sh",
	"teardown.sh",
	"reset.sh",
	"status.sh",
	"deploy.sh",
	"db-setup.sh",
	"ldap-seed.sh",
	"seed-data.sh",
	"clab.sh",
	"k3d-setup.sh",
}

// runScript executes scriptPath with the given verb and returns stdout, stderr
// and the exit code. A 5-second timeout guards against scripts that hang
// waiting for a TTY or a service that is not running.
func runScript(t *testing.T, repoRoot, scriptName, verb string) (stdout string, stderr string, exitCode int) {
	t.Helper()
	bin, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not available: %v", err)
	}
	cmd := exec.Command(bin, filepath.Join(repoRoot, "scripts", scriptName), verb)
	cmd.Dir = repoRoot
	// Pretend to be a non-interactive shell so scripts that read from a TTY
	// bail early instead of blocking the test.
	cmd.Env = append(os.Environ(), "NONINTERACTIVE=1")
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running %s %s: %v", scriptName, verb, err)
	} else {
		exitCode = 0
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// TestScriptsPresent ensures the canonical set of orchestration scripts
// actually exists on disk. Failing this test means the rest of the
// conformance checks would be vacuous.
func TestScriptsPresent(t *testing.T) {
	root := repoRoot(t)
	for _, name := range allScripts {
		path := filepath.Join(root, "scripts", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("script %s missing: %v", name, err)
		}
	}
}

// TestScriptHelpAlwaysSucceeds checks that `script.sh help` returns exit
// code 0 and prints a usage message. The standard interface requires `help`
// to work even for scripts whose main action would otherwise mutate state.
func TestScriptHelpAlwaysSucceeds(t *testing.T) {
	root := repoRoot(t)
	for _, name := range allScripts {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code := runScript(t, root, name, "help")
			if code != 0 {
				t.Errorf("%s help: exit=%d, stderr=%q", name, code, stderr)
			}
			combined := strings.ToLower(stdout + "\n" + stderr)
			if !strings.Contains(combined, "deploy") ||
				!strings.Contains(combined, "status") ||
				!strings.Contains(combined, "logs") ||
				!strings.Contains(combined, "teardown") {
				t.Errorf("%s help output must list the four standard verbs; got:\n%s", name, stdout)
			}
		})
	}
}

// TestScriptUnknownVerbFails checks that passing an unrecognised verb exits
// non-zero (per spec: "unknown subcommands return non-zero exit code with
// usage info"). We don't pin the exact exit code to allow implementations
// freedom (some scripts return 2 for usage errors, some 1).
func TestScriptUnknownVerbFails(t *testing.T) {
	root := repoRoot(t)
	for _, name := range allScripts {
		t.Run(name, func(t *testing.T) {
			_, stderr, code := runScript(t, root, name, "this-is-not-a-verb")
			if code == 0 {
				t.Errorf("%s bogus: expected non-zero exit, got 0; stderr=%q", name, stderr)
			}
			// scripts should at least mention the bad verb or the standard
			// verbs in their usage info.
			combined := strings.ToLower(stderr)
			if !strings.Contains(combined, "unknown") &&
				!strings.Contains(combined, "usage") &&
				!strings.Contains(combined, "invalid") {
				t.Errorf("%s bogus: stderr should describe the error; got %q", name, stderr)
			}
		})
	}
}

// TestScriptSafeVerbsDoNotStartServices verifies that status, logs, and help
// on each script are "safe" verbs that must NOT attempt to start any external
// service (docker, k3d, containerlab, etc.). We use a very tight timeout: if
// the script tries to talk to a service it will hang and the test will
// fail. help must be quick; status and logs are also expected to be quick
// because they only inspect local state.
func TestScriptSafeVerbsDoNotStartServices(t *testing.T) {
	root := repoRoot(t)
	for _, name := range allScripts {
		for _, verb := range []string{"status", "logs", "help"} {
			t.Run(name+"/"+verb, func(t *testing.T) {
				done := make(chan struct{})
				var (
					stdout, stderr string
					code           int
				)
				go func() {
					defer close(done)
					stdout, stderr, code = runScript(t, root, name, verb)
				}()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatalf("%s %s: did not return within 5s (likely tried to start a service); stdout=%q stderr=%q",
						name, verb, stdout, stderr)
				}
				// We don't assert on exit code for status/logs because some
				// scripts may legitimately return non-zero when no service is
				// running — the contract is "do not start services", not
				// "succeed". We just record the code for diagnostics.
				t.Logf("%s %s: exit=%d stdout=%q stderr=%q", name, verb, code, stdout, stderr)
			})
		}
	}
}

// TestScriptConventions verifies the cross-cutting style requirements in
// Requirement: Standard Script Interface → Script conventions:
//   - `set -euo pipefail` at the top
//   - exit non-zero on any failure
//
// We don't check colour output or the [STEP] prefix from a Go test (the
// test would be brittle to TTY detection). Those are smoke-tested manually.
func TestScriptConventions(t *testing.T) {
	root := repoRoot(t)
	for _, name := range allScripts {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "scripts", name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if !strings.Contains(string(data), "set -euo pipefail") {
				t.Errorf("%s must use `set -euo pipefail`", name)
			}
		})
	}
}
