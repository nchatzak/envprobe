#!/usr/bin/env zsh
# Pre-commit checks for envprobe. Run from anywhere: ./scripts/check.sh
set -u
cd "$(dirname "$0")/.." || exit 1

failed=0

run() {
	local label=$1
	shift
	print -n "  $label ... "
	if output=$("$@" 2>&1); then
		print "ok"
	else
		print "FAILED"
		print -r -- "$output" | sed 's/^/    /'
		failed=1
		return 1
	fi
}

print "envprobe checks"

# gofmt -l exits 0 even when files need formatting, so check for output instead.
print -n "  gofmt ... "
unformatted=$(gofmt -l .)
if [[ -n "$unformatted" ]]; then
	print "FAILED"
	print -r -- "$unformatted" | sed 's/^/    needs formatting: /'
	failed=1
else
	print "ok"
fi

run "build " go build ./...
run "vet   " go vet ./...
rm -f coverage.out
if run "test  " go test -race -coverprofile=coverage.out ./...; then
	# run() swallows go test's per-package coverage on success, so read the
	# total back out of the profile.
	print "  cover  ... $(go tool cover -func=coverage.out | tail -1 | awk '{print $NF}')"
fi

# CI always lints, so a missing golangci-lint is a gap in this run, not an
# opt-out.
lint_skipped=0
if (( $+commands[golangci-lint] )); then
	run "lint  " golangci-lint run
else
	print "  lint   ... SKIPPED"
	print "    golangci-lint is not installed"
	lint_skipped=1
fi

if (( failed )); then
	print "\nchecks failed"
	exit 1
fi

if (( lint_skipped )); then
	print "\nall checks passed, but lint did not run"
else
	print "\nall checks passed"
fi
