#!/usr/bin/env bash
#
# Asserts envprobe's exit-code contract against a real built binary:
#
#   0  every check passed, or there was nothing to check
#   1  the checks ran and at least one failed (--ci only)
#   2  envprobe could not check
#
# This is the automated form of test_steps.md. It runs the shipped binary
# rather than a test one, so it is the only thing covering main, os.Exit and
# Execute -- and the only place the real config search path is exercised.
#
# Usage: scripts/exit-codes.sh [path-to-binary]   (default: ./envprobe)

set -euo pipefail

bin=${1:-./envprobe}
[ -x "$bin" ] || { echo "not executable: $bin" >&2; exit 1; }
bin=$(cd "$(dirname "$bin")" && pwd)/$(basename "$bin")

# Resolved, because mktemp -d hands back a path through a symlink on macOS and
# the assertions below compare against the path viper reports.
work=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$work"' EXIT

# Isolate from the developer's own config: the search path is cwd, then $HOME,
# then $HOME/.config/envprobe, so both have to move.
export HOME="$work/home"
mkdir -p "$HOME/.config/envprobe"
cd "$work"

failures=0
rc=0
stdout=
stderr=

run() {
	set +e
	stdout=$("$bin" "$@" 2>"$work/.stderr")
	rc=$?
	set -e
	stderr=$(cat "$work/.stderr")
}

# expect <want-exit> <description> -- <args...>
expect() {
	local want=$1 desc=$2
	shift 3 # want, desc, and the -- separator
	run "$@"
	if [ "$rc" -eq "$want" ]; then
		printf 'ok   %-44s exit=%d\n' "$desc" "$rc"
	else
		printf 'FAIL %-44s exit=%d want=%d\n' "$desc" "$rc" "$want"
		printf '     stdout: %s\n     stderr: %s\n' "$stdout" "$stderr"
		failures=$((failures + 1))
	fi
}

# Asserts on the streams left behind by the most recent expect.
assert_stdout_empty() {
	[ -z "$stdout" ] && return 0
	printf 'FAIL %-44s stdout not empty:\n%s\n' "$1" "$stdout"
	failures=$((failures + 1))
}

assert_stdout_is() {
	[ "$stdout" = "$2" ] && return 0
	printf 'FAIL %-44s stdout=%s want=%s\n' "$1" "$stdout" "$2"
	failures=$((failures + 1))
}

assert_contains() {
	case "$2" in
	*"$3"*) return 0 ;;
	esac
	printf 'FAIL %-44s missing %s in:\n%s\n' "$1" "$3" "$2"
	failures=$((failures + 1))
}

assert_not_contains() {
	case "$2" in
	*"$3"*)
		printf 'FAIL %-44s unexpected %s in:\n%s\n' "$1" "$3" "$2"
		failures=$((failures + 1))
		;;
	esac
}

assert_no_usage() {
	case "$stdout$stderr" in
	*Usage:*)
		printf 'FAIL %-44s printed usage for a runtime failure\n' "$1"
		failures=$((failures + 1))
		;;
	esac
}

echo "== no config file =="
expect 2 "doctor" -- doctor
assert_stdout_empty "doctor"
assert_contains "doctor" "$stderr" "no config file found"
# Nothing was read, so there is no file to name.
assert_not_contains "doctor" "$stderr" "using "
assert_no_usage "doctor"

# Empty stdout here proves the loader failed before --json was ever read: a
# consumer must not be handed something parseable when nothing ran.
expect 2 "doctor --json" -- doctor --json
assert_stdout_empty "doctor --json"

expect 2 "config validate" -- config validate

echo "== config with an empty check list =="
echo 'checks: []' >envprobe.yaml
expect 0 "doctor" -- doctor
assert_contains "doctor" "$stderr" "no checks configured"
# Which file the search picked, as one string: two substrings would pass on a
# path from somewhere else. On stderr, so --json stays parseable -- the stdout
# assertion below is what proves it did not leak.
assert_contains "doctor" "$stderr" "using $work/envprobe.yaml"
expect 0 "doctor --json" -- doctor --json
assert_stdout_is "doctor --json" "[]"
expect 0 "config validate" -- config validate
# A gate that verifies nothing is broken, not passing.
expect 2 "doctor --ci" -- doctor --ci
assert_stdout_empty "doctor --ci"

echo "== a failing check =="
# binary is exec.LookPath, which stats rather than spawning, and version
# detection only runs on a hit -- so this stays process-free.
printf 'checks:\n  - name: definitely-not-installed-xyzzy\n    type: binary\n' >envprobe.yaml
expect 0 "doctor" -- doctor
expect 1 "doctor --ci" -- doctor --ci
assert_no_usage "doctor --ci"
# The tally is a promise the README makes about any log, so assert it against
# the real binary and not just the command under test.
assert_contains "doctor --ci" "$stderr" "0 of 1 checks passed"

echo "== config envprobe cannot build =="
printf 'checks:\n  - name: x\n    type: nosuchtype\n' >envprobe.yaml
expect 2 "unknown check type" -- doctor
assert_contains "unknown check type" "$stderr" "nosuchtype"
# A file was read, so the run names it even though the build failed.
assert_contains "unknown check type" "$stderr" "using $work/envprobe.yaml"
printf 'checks: [\n' >envprobe.yaml
expect 2 "malformed yaml" -- doctor
# Named by the error rather than the source line: the read failed, so
# configuredChecks never got a viper to report a path from. Asserting both
# halves, since the split is the design.
assert_contains "malformed yaml" "$stderr" "$work/envprobe.yaml"
assert_not_contains "malformed yaml" "$stderr" "using "

echo "== misuse =="
rm -f envprobe.yaml
expect 2 "unknown flag" -- doctor --nope
# stderr, not stdout: cobra's usage block goes to OutOrStderr(), and the real
# binary never calls SetOut, so it falls back to os.Stderr. The unit tests see
# it on stdout only because they set an out buffer.
assert_contains "unknown flag" "$stderr" "Usage:"
assert_stdout_empty "unknown flag"
expect 2 "unexpected argument" -- config example extra-arg

echo "== success paths =="
expect 0 "--help" -- --help
expect 0 "config example" -- config example
expect 0 "config init" -- config init
expect 2 "config init over existing file" -- config init
expect 0 "config init --force" -- config init --force

echo
if [ "$failures" -ne 0 ]; then
	echo "$failures assertion(s) failed"
	exit 1
fi
echo "exit-code contract OK"
