#!/usr/bin/env zsh
# Pre-commit checks for envprobe. Run from anywhere: ./scripts/check.sh
set -u
cd "$(dirname "$0")/.."

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
run "test  " go test -race -cover ./...

# Optional: only runs if you have it installed.
if (( $+commands[golangci-lint] )); then
	run "lint  " golangci-lint run
fi

if (( failed )); then
	print "\nchecks failed"
	exit 1
fi

print "\nall checks passed"
