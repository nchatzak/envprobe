#!/usr/bin/env bash
# CRAP gate. crap4go is the tool. No fallbacks.
#
#   CRAP(fn) = CC² x (1 - coverage)³ + CC
#
# Runs at REVIEW time, not on every implementer stop — crap4go re-runs the
# whole suite, so it's too slow for the fast gate.
#
# Two modes:
#   ./scripts/crap.sh baseline   snapshot current scores
#   ./scripts/crap.sh            score and gate against that snapshot
#
# Both modes print a summary. The full crap4go table goes to .pipeline.
#
# The baseline is what makes a tight threshold usable on a real codebase.
# Functions the diff made worse are REGRESSIONS and fail. Functions that were
# already bad are PRE-EXISTING: reported for the simplifier, not blocking.
#
# Positional args and CRAP_FILTER are path fragments. Fragments that match
# nothing at all exit 2. A fragment that matches nothing alongside one that
# does is not detectable here and passes.
#
# Env:
#   CRAP_THRESHOLD   default 10   fail above this
#   CRAP_WARN        default 6    report between warn and threshold
#   CRAP_TEST_CMD    passed to --test-command
#   CRAP_WORKERS     passed to --max-workers
#   CRAP_FILTER      path fragments to scope the report

set -uo pipefail

THRESHOLD="${CRAP_THRESHOLD:-10}"
WARN="${CRAP_WARN:-6}"
PIPE_DIR=".pipeline"
REPORT="$PIPE_DIR/crap.txt"
BASELINE="$PIPE_DIR/crap-baseline.txt"
mkdir -p "$PIPE_DIR"

MODE="gate"
[ "${1:-}" = "baseline" ] && { MODE="baseline"; shift; }

command -v go >/dev/null 2>&1 || { echo "ERROR: go toolchain not found" >&2; exit 2; }

GOBIN="$(go env GOBIN)"; [ -n "$GOBIN" ] || GOBIN="$(go env GOPATH)/bin"
export PATH="$GOBIN:$PATH"

if ! command -v crap4go >/dev/null 2>&1; then
  echo "  crap4go not found; installing..."
  go install github.com/unclebob/crap4go/cmd/crap4go@latest || {
    echo "ERROR: failed to install crap4go" >&2; exit 2; }
  command -v crap4go >/dev/null 2>&1 || {
    echo "ERROR: crap4go not on PATH. Add $GOBIN to PATH." >&2; exit 2; }
fi

FILTERED=0
ARGS=()
[ -n "${CRAP_TEST_CMD:-}" ] && ARGS+=(--test-command "$CRAP_TEST_CMD")
[ -n "${CRAP_WORKERS:-}" ]  && ARGS+=(--max-workers "$CRAP_WORKERS")
if [ "$#" -gt 0 ]; then ARGS+=("$@"); FILTERED=1
elif [ -n "${CRAP_FILTER:-}" ]; then
  # shellcheck disable=SC2206
  ARGS+=(${CRAP_FILTER}); FILTERED=1
fi

# Baseline writes to a temp and moves it into place only after the summary
# exits 0.
OUT="$REPORT"
[ "$MODE" = "baseline" ] && OUT="$BASELINE.tmp"

crap_failed() {
  cat "$OUT" >&2
  echo "ERROR: crap4go failed" >&2
  [ "$MODE" = "baseline" ] && rm -f "$OUT"
  exit 2
}

# macOS ships bash 3.2, where "${arr[@]}" on an EMPTY array is an
# unbound-variable error under `set -u`. ARGS is empty in the common case
# (no test command, no workers, no filter), so this must be branched.
if [ ${#ARGS[@]} -gt 0 ]; then
  crap4go "${ARGS[@]}" > "$OUT" 2>&1 || crap_failed
else
  crap4go > "$OUT" 2>&1 || crap_failed
fi

# Report rows: Function Package CC Cov% CRAP  (neither name contains spaces)
#
# Two files in, baseline then report. Baseline mode passes its snapshot as
# both, so every function is known at its own score.
SUMMARY='
  function isnum(x) { return x ~ /^[0-9]+(\.[0-9]+)?$/ }

  NR == FNR {                                   # baseline
    if (isnum($NF)) base[$2 "." $1] = $NF + 0
    next
  }
  {
    if ($NF == "N/A" || $NF == "n/a") {
      na++
      nas = nas sprintf("    %-28s %s\n", $1, $2)
      next
    }
    if (!isnum($NF)) next
    k = $2 "." $1
    c = $NF + 0
    n++; total += c
    known = (k in base)
    prev = known ? base[k] : 0

    if (c > t) {
      if (!known || c > prev + 0.05) {
        reg++
        regs = regs sprintf("    %-28s %-18s %6.1f%s\n", $1, $2,
                 c, known ? sprintf("  (was %.1f)", prev) : "  (new)")
      } else {
        pre++
        pres = pres sprintf("    %-28s %-18s %6.1f\n", $1, $2, c)
      }
    } else if (c > w) {
      warn++
      warns = warns sprintf("    %-28s %-18s %6.1f\n", $1, $2, c)
    }
  }
  END {
    baseline = (mode == "baseline")

    if (n == 0 && na == 0) {
      if (filtered) {
        print "\n  ERROR: the given path fragments matched no scored functions." > "/dev/stderr"
        print "  Nothing was scored. Fix the paths passed to crap.sh." > "/dev/stderr"
        exit 2
      }
      printf "\n  SKIP: no scored functions (full output: %s)\n", report
      exit 0
    }

    printf "\n  %s: %d   average CRAP: %.1f   warn: >%s   fail: >%s\n",
           baseline ? "baseline snapshot" : "scored",
           n, (n ? total/n : 0), w, t
    printf "  full table: %s\n", report

    if (na > 0) {
      printf "\n  %d N/A - unmeasured, not clean:\n", na
      printf "%s", nas
    }
    if (warn > 0) {
      printf "\n  %d in warn band (%s-%s):\n", warn, w, t
      printf "%s", warns
    }
    if (pre > 0) {
      hdr = "PRE-EXISTING over threshold (not blocking; simplifier worklist):"
      if (baseline) hdr = "already over threshold (the implementer is not held to these):"
      printf "\n  %s\n", hdr
      printf "%s", pres
    }

    if (baseline) exit 0

    if (reg > 0) {
      printf "\n  REGRESSIONS - this diff made these worse or added them:\n"
      printf "%s", regs
      printf "\n  These are the implementers responsibility: write it simpler,\n"
      printf "  or add the tests the plan called for. Do not refactor\n"
      printf "  pre-existing code to compensate.\n"
      exit 1
    }
    print "\n  no CRAP regressions in this diff"
    exit 0
  }
'

if [ "$MODE" = "baseline" ]; then
  awk -v t="$THRESHOLD" -v w="$WARN" -v report="$BASELINE" \
      -v mode=baseline -v filtered="$FILTERED" "$SUMMARY" "$OUT" "$OUT"
  STATUS=$?
  [ "$STATUS" -eq 0 ] || { rm -f "$OUT"; exit "$STATUS"; }
  mv "$OUT" "$BASELINE"
  exit 0
fi

# With no baseline, compare against a placeholder rather than creating
# $BASELINE: only baseline mode writes that file, so an existence check on it
# still means a snapshot was taken. The placeholder needs a record, because a
# zero-length first file leaves NR == FNR true through the report and every
# row is read as baseline.
BASE_IN="$BASELINE"
if [ ! -s "$BASELINE" ]; then
  BASE_IN="$PIPE_DIR/crap-nobaseline.txt"
  echo "CRAP baseline: none taken" > "$BASE_IN"
fi

awk -v t="$THRESHOLD" -v w="$WARN" -v report="$REPORT" \
    -v mode=gate -v filtered="$FILTERED" "$SUMMARY" "$BASE_IN" "$REPORT"
